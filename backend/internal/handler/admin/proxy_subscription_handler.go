package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ssutil"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

const (
	subscriptionFetchTimeout = 20 * time.Second
	subscriptionMaxBodyBytes = int64(8 * 1024 * 1024)

	subscriptionDefaultUserAgent = "sub2api-subscription-parser/1.0"
	subscriptionClashUserAgent   = "Clash"
)

type ParseSubscriptionRequest struct {
	URL string `json:"url" binding:"required"`
}

type ParsedSubscriptionProxy struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type ParseSubscriptionResponse struct {
	Proxies []ParsedSubscriptionProxy `json:"proxies"`
}

func (h *ProxyHandler) ParseSubscription(c *gin.Context) {
	var req ParseSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	validatedURL, err := urlvalidator.ValidateHTTPURL(req.URL, false, urlvalidator.ValidationOptions{
		AllowPrivate: false,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	proxies, err := fetchAndParseSubscription(c.Request.Context(), validatedURL)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, ParseSubscriptionResponse{Proxies: proxies})
}

func fetchAndParseSubscription(ctx context.Context, rawURL string) ([]ParsedSubscriptionProxy, error) {
	body, err := fetchSubscriptionBody(ctx, rawURL, subscriptionDefaultUserAgent)
	if err != nil {
		return nil, err
	}

	proxies, parseErr := parseSubscriptionBody(body)
	if parseErr == nil && len(proxies) > 0 {
		return proxies, nil
	}

	body, err = fetchSubscriptionBody(ctx, rawURL, subscriptionClashUserAgent)
	if err != nil {
		if parseErr != nil {
			return nil, parseErr
		}
		return nil, err
	}

	proxies, err = parseSubscriptionBody(body)
	if err != nil {
		if parseErr != nil {
			return nil, parseErr
		}
		return nil, err
	}
	return proxies, nil
}

func fetchSubscriptionBody(ctx context.Context, rawURL, userAgent string) ([]byte, error) {
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:            subscriptionFetchTimeout,
		ValidateResolvedIP: true,
		AllowPrivateHosts:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("build subscription client: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build subscription request: %w", err)
	}
	req.Header.Set("Accept", "text/plain, application/yaml, text/yaml, */*")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, subscriptionMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read subscription body failed: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("subscription response is empty")
	}
	return body, nil
}

func parseSubscriptionBody(body []byte) ([]ParsedSubscriptionProxy, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, errors.New("subscription response is empty")
	}

	if proxies, err := parseSubscriptionText(trimmed); err == nil && len(proxies) > 0 {
		return proxies, nil
	}

	if proxies, err := parseClashSubscriptionYAML([]byte(trimmed)); err == nil && len(proxies) > 0 {
		return proxies, nil
	}

	decoded, err := decodeSubscriptionBase64(trimmed)
	if err == nil && !bytes.Equal(decoded, body) {
		if proxies, err := parseSubscriptionBody(decoded); err == nil && len(proxies) > 0 {
			return proxies, nil
		}
	}

	return nil, errors.New("no supported proxies found in subscription")
}

func parseSubscriptionText(raw string) ([]ParsedSubscriptionProxy, error) {
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	if len(lines) == 0 {
		return nil, errors.New("subscription text is empty")
	}

	proxies := make([]ParsedSubscriptionProxy, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(strings.ToLower(line), "ss://") {
			continue
		}

		node, err := ssutil.ParseURL(line)
		if err != nil || node == nil || node.Plugin != "" {
			continue
		}

		proxies = append(proxies, ParsedSubscriptionProxy{
			Name:     subscriptionProxyName(node.Tag, node.Host, node.Port),
			Protocol: "ss",
			Host:     node.Host,
			Port:     node.Port,
			Username: node.Method,
			Password: node.Password,
		})
	}

	if len(proxies) == 0 {
		return nil, errors.New("no supported ss proxies found")
	}
	return proxies, nil
}

func parseClashSubscriptionYAML(body []byte) ([]ParsedSubscriptionProxy, error) {
	type clashProxy struct {
		Name     string `yaml:"name"`
		Type     string `yaml:"type"`
		Server   string `yaml:"server"`
		Port     int    `yaml:"port"`
		Cipher   string `yaml:"cipher"`
		Password string `yaml:"password"`
		Plugin   string `yaml:"plugin"`
	}
	type clashConfig struct {
		Proxies []clashProxy `yaml:"proxies"`
	}

	var cfg clashConfig
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Proxies) == 0 {
		return nil, errors.New("clash subscription contains no proxies")
	}

	proxies := make([]ParsedSubscriptionProxy, 0, len(cfg.Proxies))
	for _, item := range cfg.Proxies {
		if strings.ToLower(strings.TrimSpace(item.Type)) != "ss" {
			continue
		}
		if strings.TrimSpace(item.Plugin) != "" {
			continue
		}

		host := strings.TrimSpace(item.Server)
		method := strings.TrimSpace(item.Cipher)
		password := strings.TrimSpace(item.Password)
		if host == "" || method == "" || password == "" || item.Port <= 0 || item.Port > 65535 {
			continue
		}

		proxies = append(proxies, ParsedSubscriptionProxy{
			Name:     subscriptionProxyName(item.Name, host, item.Port),
			Protocol: "ss",
			Host:     host,
			Port:     item.Port,
			Username: method,
			Password: password,
		})
	}

	if len(proxies) == 0 {
		return nil, errors.New("clash subscription contains no supported ss proxies")
	}
	return proxies, nil
}

func decodeSubscriptionBase64(raw string) ([]byte, error) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, raw)
	if compact == "" {
		return nil, errors.New("empty base64 payload")
	}

	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(compact)
		if err == nil {
			return decoded, nil
		}
	}

	padded := compact + strings.Repeat("=", (4-len(compact)%4)%4)
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(padded)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid base64 payload")
}

func subscriptionProxyName(name, host string, port int) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s:%d", strings.TrimSpace(host), port)
}
