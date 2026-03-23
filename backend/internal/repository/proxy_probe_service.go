package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func NewProxyExitInfoProber(cfg *config.Config) service.ProxyExitInfoProber {
	insecure := false
	allowPrivate := false
	validateResolvedIP := true
	maxResponseBytes := defaultProxyProbeResponseMaxBytes
	if cfg != nil {
		insecure = cfg.Security.ProxyProbe.InsecureSkipVerify
		allowPrivate = cfg.Security.URLAllowlist.AllowPrivateHosts
		validateResolvedIP = cfg.Security.URLAllowlist.Enabled
		if cfg.Gateway.ProxyProbeResponseReadMaxBytes > 0 {
			maxResponseBytes = cfg.Gateway.ProxyProbeResponseReadMaxBytes
		}
	}
	if insecure {
		log.Printf("[ProxyProbe] Warning: insecure_skip_verify is not allowed and will cause probe failure.")
	}
	return &proxyProbeService{
		insecureSkipVerify: insecure,
		allowPrivateHosts:  allowPrivate,
		validateResolvedIP: validateResolvedIP,
		maxResponseBytes:   maxResponseBytes,
	}
}

const (
	defaultProxyProbeTimeout          = 10 * time.Second
	defaultProxyProbeResponseMaxBytes = int64(1024 * 1024)
	proxyProbeUserAgent               = "sub2api-proxy-probe/1.0"
)

// probeURLs 按优先级排列的探测 URL 列表
// 使用 HTTPS 地理信息源，尽量减少公共 HTTP 出口探测服务的不稳定因素。
var probeURLs = []struct {
	url    string
	parser string // "country-is" or "ifconfig"
}{
	{"https://api.country.is/?fields=ip,country,city,subdivision", "country-is"},
	{"https://ifconfig.co/json", "ifconfig"},
}

type proxyProbeService struct {
	insecureSkipVerify bool
	allowPrivateHosts  bool
	validateResolvedIP bool
	maxResponseBytes   int64
}

func (s *proxyProbeService) ProbeProxy(ctx context.Context, proxyURL string) (*service.ProxyExitInfo, int64, error) {
	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:           proxyURL,
		Timeout:            defaultProxyProbeTimeout,
		InsecureSkipVerify: s.insecureSkipVerify,
		ValidateResolvedIP: s.validateResolvedIP,
		AllowPrivateHosts:  s.allowPrivateHosts,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create proxy client: %w", err)
	}

	var lastErr error
	for _, probe := range probeURLs {
		exitInfo, latencyMs, err := s.probeWithURL(ctx, client, probe.url, probe.parser)
		if err == nil {
			return exitInfo, latencyMs, nil
		}
		lastErr = err
	}

	return nil, 0, fmt.Errorf("all probe URLs failed, last error: %w", lastErr)
}

func (s *proxyProbeService) probeWithURL(ctx context.Context, client *http.Client, url string, parser string) (*service.ProxyExitInfo, int64, error) {
	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", proxyProbeUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("proxy connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	latencyMs := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		return nil, latencyMs, fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	maxResponseBytes := s.maxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultProxyProbeResponseMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, latencyMs, fmt.Errorf("failed to read response: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, latencyMs, fmt.Errorf("proxy probe response exceeds limit: %d", maxResponseBytes)
	}

	switch parser {
	case "country-is":
		return s.parseCountryIs(body, latencyMs)
	case "ifconfig":
		return s.parseIfConfig(body, latencyMs)
	default:
		return nil, latencyMs, fmt.Errorf("unknown parser: %s", parser)
	}
}

func (s *proxyProbeService) parseCountryIs(body []byte, latencyMs int64) (*service.ProxyExitInfo, int64, error) {
	fields, err := parseProbeJSON(body)
	if err != nil {
		return nil, latencyMs, err
	}

	if success, ok := fields["success"].(bool); ok && !success {
		message := firstJSONString(fields, "message", "error", "detail")
		if message == "" {
			message = "country.is request failed"
		}
		return nil, latencyMs, fmt.Errorf("country.is request failed: %s", message)
	}

	ipAddress := firstJSONString(fields, "ip", "query")
	if ipAddress == "" {
		return nil, latencyMs, fmt.Errorf("country.is: no IP found in response")
	}

	countryCode := strings.ToUpper(firstJSONString(fields, "country_code", "country"))
	country := firstJSONString(fields, "country_name")
	if country == "" {
		country = countryCode
	}

	return &service.ProxyExitInfo{
		IP:          ipAddress,
		City:        firstJSONString(fields, "city"),
		Region:      firstJSONString(fields, "subdivision", "region_name", "region"),
		Country:     country,
		CountryCode: countryCode,
	}, latencyMs, nil
}

func (s *proxyProbeService) parseIfConfig(body []byte, latencyMs int64) (*service.ProxyExitInfo, int64, error) {
	fields, err := parseProbeJSON(body)
	if err != nil {
		return nil, latencyMs, err
	}

	if firstJSONString(fields, "error", "detail", "message") != "" && firstJSONString(fields, "ip") == "" {
		return nil, latencyMs, fmt.Errorf("ifconfig probe failed: %s", firstJSONString(fields, "error", "detail", "message"))
	}

	ipAddress := firstJSONString(fields, "ip")
	if ipAddress == "" {
		return nil, latencyMs, fmt.Errorf("ifconfig: no IP found in response")
	}

	countryCode := strings.ToUpper(firstJSONString(fields, "country_iso", "country_code"))
	country := firstJSONString(fields, "country")
	if country == "" {
		country = countryCode
	}

	return &service.ProxyExitInfo{
		IP:          ipAddress,
		City:        firstJSONString(fields, "city"),
		Region:      firstJSONString(fields, "region_name", "region", "subdivision"),
		Country:     country,
		CountryCode: countryCode,
	}, latencyMs, nil
}

func parseProbeJSON(body []byte) (map[string]any, error) {
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("failed to parse response: %w (body: %s)", err, preview)
	}
	return fields, nil
}

func firstJSONString(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		if text := extractProbeString(value); text != "" {
			return text
		}
	}
	return ""
}

func extractProbeString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		for _, key := range []string{"name", "value", "code", "iso", "iso_code"} {
			nested, ok := v[key]
			if !ok {
				continue
			}
			if text := extractProbeString(nested); text != "" {
				return text
			}
		}
	}
	return ""
}
