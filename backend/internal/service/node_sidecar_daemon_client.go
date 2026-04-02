package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type nodeSidecarDaemonClient struct {
	socketPath string
	scriptPath string
	timeout    time.Duration

	mu  sync.Mutex
	cmd *exec.Cmd
}

type sidecarDaemonRequest struct {
	ClientMode    string              `json:"client_mode,omitempty"`
	Method        string              `json:"method"`
	Endpoint      string              `json:"endpoint"`
	Headers       map[string][]string `json:"headers,omitempty"`
	PayloadBase64 string              `json:"payload_base64,omitempty"`
	TimeoutMS     int                 `json:"timeout_ms"`
	AcceptNon2xx  bool                `json:"accept_non_2xx"`
	ReturnRaw     bool                `json:"return_raw_bytes"`
	ProxyURL      string              `json:"proxy_url,omitempty"`
	Stream        bool                `json:"stream_response,omitempty"`
}

type daemonBufferedResponse struct {
	Status     int                 `json:"status"`
	Error      string              `json:"error"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64"`
}

type daemonStreamBody struct {
	conn   net.Conn
	reader *bufio.Reader
}

func (b *daemonStreamBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *daemonStreamBody) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

func newNodeSidecarDaemonClient(cfg *config.Config) (*nodeSidecarDaemonClient, error) {
	if cfg == nil || !cfg.Gateway.SidecarDaemon.Enabled {
		return nil, nil
	}
	socketPath := strings.TrimSpace(cfg.Gateway.SidecarDaemon.SocketPath)
	if socketPath == "" {
		socketPath = "/tmp/sub2api-node-sidecar.sock"
	}
	startTimeout := time.Duration(cfg.Gateway.SidecarDaemon.StartupTimeoutSeconds) * time.Second
	if startTimeout <= 0 {
		startTimeout = 5 * time.Second
	}
	scriptPath, err := findNodeSidecarDaemonScript()
	if err != nil {
		return nil, err
	}
	return &nodeSidecarDaemonClient{
		socketPath: socketPath,
		scriptPath: scriptPath,
		timeout:    startTimeout,
	}, nil
}

func findNodeSidecarDaemonScript() (string, error) {
	if override := strings.TrimSpace(os.Getenv("SUB2API_SIDECAR_DAEMON_SCRIPT")); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("sidecar daemon script not found at %s", override)
	}
	candidates := []string{
		filepath.Join("..", "tools", "sidecar-daemon.mjs"),
		filepath.Join("tools", "sidecar-daemon.mjs"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("sidecar daemon script not found")
}

func (c *nodeSidecarDaemonClient) ensureStarted(ctx context.Context) error {
	if c == nil {
		return errors.New("nil sidecar daemon client")
	}
	if conn, err := net.DialTimeout("unix", c.socketPath, 250*time.Millisecond); err == nil {
		_ = conn.Close()
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, err := net.DialTimeout("unix", c.socketPath, 250*time.Millisecond); err == nil {
		_ = conn.Close()
		return nil
	}

	_ = os.Remove(c.socketPath)
	cmd := exec.CommandContext(context.Background(), "node", c.scriptPath, "--socket", c.socketPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sidecar daemon: %w", err)
	}
	c.cmd = cmd
	go func(cmd *exec.Cmd) {
		_ = cmd.Wait()
	}(cmd)

	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if conn, err := net.DialTimeout("unix", c.socketPath, 250*time.Millisecond); err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("sidecar daemon did not become ready: %s", c.socketPath)
}

func (c *nodeSidecarDaemonClient) dial(ctx context.Context) (net.Conn, error) {
	if err := c.ensureStarted(ctx); err != nil {
		return nil, err
	}
	d := net.Dialer{}
	return d.DialContext(ctx, "unix", c.socketPath)
}

func (c *nodeSidecarDaemonClient) roundTripBuffered(ctx context.Context, req sidecarDaemonRequest, httpReq *http.Request) (*http.Response, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	return c.roundTripBufferedConn(conn, req, httpReq)
}

func (c *nodeSidecarDaemonClient) roundTripBufferedConn(conn net.Conn, req sidecarDaemonRequest, httpReq *http.Request) (*http.Response, error) {
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("encode daemon request: %w", err)
	}
	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("read daemon response: %w", err)
	}
	var decoded daemonBufferedResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode daemon response: %w (%s)", err, strings.TrimSpace(string(data)))
	}
	if decoded.Error != "" {
		return nil, errors.New(decoded.Error)
	}
	bodyBytes, err := decodeDaemonBody(decoded.BodyBase64)
	if err != nil {
		return nil, err
	}
	resp := &http.Response{
		StatusCode:    decoded.Status,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(bodyBytes)),
		ContentLength: int64(len(bodyBytes)),
		Request:       httpReq,
	}
	for k, vals := range decoded.Headers {
		for _, v := range vals {
			resp.Header.Add(k, v)
		}
	}
	return resp, nil
}

func (c *nodeSidecarDaemonClient) roundTripStream(ctx context.Context, req sidecarDaemonRequest, httpReq *http.Request) (*http.Response, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	return c.roundTripStreamConn(conn, req, httpReq)
}

func (c *nodeSidecarDaemonClient) roundTripStreamConn(conn net.Conn, req sidecarDaemonRequest, httpReq *http.Request) (*http.Response, error) {
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("encode daemon request: %w", err)
	}
	if unixConn, ok := conn.(*net.UnixConn); ok {
		_ = unixConn.CloseWrite()
	}
	reader := bufio.NewReader(conn)
	metaLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read daemon stream header: %w", err)
	}
	var decoded daemonBufferedResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(metaLine)), &decoded); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("decode daemon stream header: %w", err)
	}
	if decoded.Error != "" {
		_ = conn.Close()
		return nil, errors.New(decoded.Error)
	}
	resp := &http.Response{
		StatusCode:    decoded.Status,
		Header:        make(http.Header),
		Body:          &daemonStreamBody{conn: conn, reader: reader},
		ContentLength: -1,
		Request:       httpReq,
	}
	for k, vals := range decoded.Headers {
		for _, v := range vals {
			resp.Header.Add(k, v)
		}
	}
	return resp, nil
}

func decodeDaemonBody(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(b64)
}
