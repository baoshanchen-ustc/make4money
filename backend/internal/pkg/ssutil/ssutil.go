package ssutil

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

type Node struct {
	Method   string
	Password string
	Host     string
	Port     int
	Tag      string
	Plugin   string
}

func ParseURL(raw string) (*Node, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("ss url is empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid ss url: %w", err)
	}
	return ParseParsedURL(parsed)
}

func ParseParsedURL(parsed *url.URL) (*Node, error) {
	if parsed == nil {
		return nil, fmt.Errorf("ss url is nil")
	}
	if strings.ToLower(strings.TrimSpace(parsed.Scheme)) != "ss" {
		return nil, fmt.Errorf("invalid ss scheme: %s", parsed.Scheme)
	}
	if node, ok, err := parseLegacyEncodedNode(parsed); ok || err != nil {
		return node, err
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("ss url missing host")
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid ss port: %s", parsed.Port())
	}

	method, password, err := credentialsFromParsedURL(parsed)
	if err != nil {
		return nil, err
	}

	query, _ := url.ParseQuery(parsed.RawQuery)
	return &Node{
		Method:   method,
		Password: password,
		Host:     parsed.Hostname(),
		Port:     port,
		Tag:      parsed.Fragment,
		Plugin:   strings.TrimSpace(query.Get("plugin")),
	}, nil
}

func parseLegacyEncodedNode(parsed *url.URL) (*Node, bool, error) {
	if parsed == nil || parsed.User != nil {
		return nil, false, nil
	}
	if parsed.Port() != "" && parsed.Hostname() != "" {
		return nil, false, nil
	}

	payload := strings.TrimSpace(parsed.Host)
	if payload == "" {
		payload = strings.TrimSpace(parsed.Opaque)
	}
	if payload == "" {
		return nil, false, nil
	}

	decoded, err := decodeBase64(payload)
	if err != nil {
		return nil, false, nil
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" || !strings.Contains(decoded, "@") {
		return nil, false, nil
	}

	raw := decoded
	if !strings.HasPrefix(strings.ToLower(raw), "ss://") {
		raw = "ss://" + raw
	}
	legacyParsed, err := url.Parse(raw)
	if err != nil {
		return nil, true, fmt.Errorf("invalid legacy ss url: %w", err)
	}
	if legacyParsed.RawQuery == "" {
		legacyParsed.RawQuery = parsed.RawQuery
	}
	if legacyParsed.Fragment == "" {
		legacyParsed.Fragment = parsed.Fragment
	}

	node, err := ParseParsedURL(legacyParsed)
	if err != nil {
		return nil, true, err
	}
	return node, true, nil
}

func BuildURL(method, password, host string, port int, tag string) (string, error) {
	method = strings.TrimSpace(method)
	password = strings.TrimSpace(password)
	host = strings.TrimSpace(host)
	if method == "" {
		return "", fmt.Errorf("ss method is required")
	}
	if password == "" {
		return "", fmt.Errorf("ss password is required")
	}
	if host == "" {
		return "", fmt.Errorf("ss host is required")
	}
	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("invalid ss port: %d", port)
	}

	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	u := &url.URL{
		Scheme: "ss",
		User:   url.User(userinfo),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	if strings.TrimSpace(tag) != "" {
		u.Fragment = tag
	}
	return u.String(), nil
}

func DialContext(ctx context.Context, proxyURL *url.URL, network, targetAddr string) (net.Conn, error) {
	node, err := ParseParsedURL(proxyURL)
	if err != nil {
		return nil, err
	}
	if node.Plugin != "" {
		return nil, fmt.Errorf("ss plugin is not supported")
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("ss only supports tcp, got %s", network)
	}

	target := socks.ParseAddr(targetAddr)
	if target == nil {
		return nil, fmt.Errorf("invalid target address: %s", targetAddr)
	}

	cipher, err := core.PickCipher(node.Method, nil, node.Password)
	if err != nil {
		return nil, fmt.Errorf("create ss cipher: %w", err)
	}

	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(node.Host, strconv.Itoa(node.Port)))
	if err != nil {
		return nil, fmt.Errorf("connect to ss server: %w", err)
	}

	conn := cipher.StreamConn(rawConn)
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("set ss deadline: %w", err)
		}
		defer func() {
			_ = conn.SetDeadline(time.Time{})
		}()
	}

	if _, err := conn.Write(target); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write ss target: %w", err)
	}
	return conn, nil
}

func credentialsFromParsedURL(parsed *url.URL) (string, string, error) {
	if parsed == nil || parsed.User == nil {
		return "", "", fmt.Errorf("ss credentials are required")
	}

	user := strings.TrimSpace(parsed.User.Username())
	if user == "" {
		return "", "", fmt.Errorf("ss credentials are required")
	}

	if password, ok := parsed.User.Password(); ok {
		password = strings.TrimSpace(password)
		if password == "" {
			return "", "", fmt.Errorf("ss password is required")
		}
		return user, password, nil
	}

	decoded, err := decodeBase64(user)
	if err != nil {
		return "", "", fmt.Errorf("invalid ss userinfo: %w", err)
	}
	method, password, ok := strings.Cut(decoded, ":")
	if !ok || strings.TrimSpace(method) == "" || strings.TrimSpace(password) == "" {
		return "", "", fmt.Errorf("invalid ss userinfo")
	}
	return strings.TrimSpace(method), strings.TrimSpace(password), nil
}

func decodeBase64(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(raw)
		if err == nil {
			return string(decoded), nil
		}
	}
	padded := raw + strings.Repeat("=", (4-len(raw)%4)%4)
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(padded)
		if err == nil {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("invalid base64 payload")
}
