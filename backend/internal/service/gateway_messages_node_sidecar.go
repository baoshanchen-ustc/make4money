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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	defaultMessagesNodeSidecarTimeout = 30 * time.Second
)

type messagesNodeSidecarResponse struct {
	Status     int                 `json:"status"`
	Error      string              `json:"error"`
	Headers    map[string][]string `json:"headers"`
	BodyBase64 string              `json:"body_base64"`
}

type tailBuffer struct {
	buf   []byte
	limit int
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		return len(p), nil
	}
	if len(p) >= b.limit {
		b.buf = append(b.buf[:0], p[len(p)-b.limit:]...)
		return len(p), nil
	}
	if len(b.buf)+len(p) > b.limit {
		drop := len(b.buf) + len(p) - b.limit
		b.buf = append(b.buf[:0], b.buf[drop:]...)
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *tailBuffer) String() string {
	return string(b.buf)
}

type messagesNodeSidecarStreamBody struct {
	reader  *bufio.Reader
	stdout  io.Closer
	cancel  context.CancelFunc
	waitCh  chan error
	waitErr error
	waited  bool
}

func (b *messagesNodeSidecarStreamBody) wait() error {
	if b == nil || b.waited {
		return b.waitErr
	}
	b.waited = true
	if b.waitCh != nil {
		b.waitErr = <-b.waitCh
	}
	return b.waitErr
}

func (b *messagesNodeSidecarStreamBody) Read(p []byte) (int, error) {
	if b == nil || b.reader == nil {
		return 0, io.EOF
	}
	n, err := b.reader.Read(p)
	if err == io.EOF {
		if waitErr := b.wait(); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}

func (b *messagesNodeSidecarStreamBody) Close() error {
	if b == nil {
		return nil
	}
	if b.cancel != nil {
		b.cancel()
	}
	if b.stdout != nil {
		_ = b.stdout.Close()
	}
	return b.wait()
}

func (s *GatewayService) shouldUseMessagesNodeSidecar(account *Account, reqStream bool) bool {
	if s == nil || s.cfg == nil {
		return false
	}
	cfg := s.cfg.Gateway.MessagesNodeSidecar
	if !cfg.Enabled {
		return false
	}
	if reqStream && !cfg.EnableStreaming {
		return false
	}
	if account == nil {
		return false
	}
	// 当前仅用于 /v1/messages 的 Anthropic 转发路径。
	return account.Platform == PlatformAnthropic
}

func (s *GatewayService) messagesNodeSidecarTimeout() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.Gateway.MessagesNodeSidecar.TimeoutSeconds <= 0 {
		return defaultMessagesNodeSidecarTimeout
	}
	return time.Duration(s.cfg.Gateway.MessagesNodeSidecar.TimeoutSeconds) * time.Second
}

func (s *GatewayService) messagesNodeSidecarAllowGoFallback() bool {
	if s == nil || s.cfg == nil {
		return true
	}
	return s.cfg.Gateway.MessagesNodeSidecar.AllowGoFallback
}

func (s *GatewayService) findMessagesNodeSidecarScript() (string, error) {
	if s != nil && s.cfg != nil {
		if script := strings.TrimSpace(s.cfg.Gateway.MessagesNodeSidecar.Script); script != "" {
			if _, err := os.Stat(script); err == nil {
				return script, nil
			}
			return "", fmt.Errorf("messages sidecar script not found at %s", script)
		}
	}
	if override := strings.TrimSpace(os.Getenv("MESSAGES_NODE_SIDECAR_SCRIPT")); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("messages sidecar script not found at %s", override)
	}

	candidates := []string{
		filepath.Join("..", "tools", "telemetry-sidecar.mjs"),
		filepath.Join("tools", "telemetry-sidecar.mjs"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("messages sidecar script not found")
}

func cloneHeaderValues(header http.Header) map[string][]string {
	if len(header) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(header))
	for key, values := range header {
		trimmed := make([]string, 0, len(values))
		for _, value := range values {
			v := strings.TrimSpace(value)
			if v == "" {
				continue
			}
			trimmed = append(trimmed, v)
		}
		if len(trimmed) > 0 {
			out[key] = trimmed
		}
	}
	return out
}

func readRequestBodyForReplay(req *http.Request) ([]byte, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	if req.GetBody != nil {
		rc, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func (s *GatewayService) doMessagesRequestWithNodeSidecar(
	ctx context.Context,
	req *http.Request,
	timeout time.Duration,
	proxyURL string,
	reqStream bool,
) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, errors.New("invalid upstream request")
	}

	scriptPath, err := s.findMessagesNodeSidecarScript()
	if err != nil {
		return nil, err
	}

	requestBody, err := readRequestBodyForReplay(req)
	if err != nil {
		return nil, fmt.Errorf("read request body for sidecar: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"client_mode":      "messages",
		"method":           req.Method,
		"endpoint":         req.URL.String(),
		"headers":          cloneHeaderValues(req.Header),
		"payload_base64":   base64.StdEncoding.EncodeToString(requestBody),
		"timeout_ms":       int(timeout / time.Millisecond),
		"accept_non_2xx":   true,
		"return_raw_bytes": true,
		"proxy_url":        strings.TrimSpace(proxyURL),
		"stream_response":  reqStream,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sidecar payload: %w", err)
	}

	if s.sidecarDaemonClient != nil {
		reqData := sidecarDaemonRequest{
			ClientMode:    "messages",
			Method:        req.Method,
			Endpoint:      req.URL.String(),
			Headers:       cloneHeaderValues(req.Header),
			PayloadBase64: base64.StdEncoding.EncodeToString(requestBody),
			TimeoutMS:     int(timeout / time.Millisecond),
			AcceptNon2xx:  true,
			ReturnRaw:     true,
			ProxyURL:      strings.TrimSpace(proxyURL),
			Stream:        reqStream,
		}
		if reqStream {
			return s.sidecarDaemonClient.roundTripStream(ctx, reqData, req)
		}
		return s.sidecarDaemonClient.roundTripBuffered(ctx, reqData, req)
	}

	if reqStream {
		return s.doStreamingMessagesRequestWithNodeSidecar(ctx, req, scriptPath, payload)
	}
	return s.doBufferedMessagesRequestWithNodeSidecar(ctx, req, scriptPath, payload, timeout)
}

func (s *GatewayService) logMessagesSidecarFallback(account *Account, err error) {
	if account == nil {
		logger.LegacyPrintf("service.gateway", "[Warn] messages node sidecar failed, fallback to Go transport: %v", err)
		return
	}
	logger.LegacyPrintf("service.gateway", "[Warn] messages node sidecar failed (account=%d, platform=%s), fallback to Go transport: %v",
		account.ID, account.Platform, err)
}

func (s *GatewayService) doBufferedMessagesRequestWithNodeSidecar(
	ctx context.Context,
	req *http.Request,
	scriptPath string,
	payload []byte,
	timeout time.Duration,
) (*http.Response, error) {
	execCtx, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "node", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("exec messages sidecar: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	var decoded messagesNodeSidecarResponse
	if err := json.Unmarshal(output, &decoded); err != nil {
		return nil, fmt.Errorf("decode messages sidecar response: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	if decoded.Error != "" {
		return nil, errors.New(decoded.Error)
	}
	bodyBytes, err := base64.StdEncoding.DecodeString(decoded.BodyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode sidecar body: %w", err)
	}
	if decoded.Status <= 0 {
		return nil, errors.New("invalid sidecar response status")
	}

	resp := &http.Response{
		StatusCode:    decoded.Status,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(bodyBytes)),
		ContentLength: int64(len(bodyBytes)),
		Request:       req,
	}
	for key, values := range decoded.Headers {
		for _, value := range values {
			resp.Header.Add(key, value)
		}
	}
	return resp, nil
}

func (s *GatewayService) doStreamingMessagesRequestWithNodeSidecar(
	ctx context.Context,
	req *http.Request,
	scriptPath string,
	payload []byte,
) (*http.Response, error) {
	execCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(execCtx, "node", scriptPath)
	cmd.Stdin = bytes.NewReader(payload)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open messages sidecar stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open messages sidecar stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start messages sidecar: %w", err)
	}

	stderrBuf := newTailBuffer(16 * 1024)
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(stderrBuf, stderr)
		close(stderrDone)
	}()

	waitCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		<-stderrDone
		if err != nil && strings.TrimSpace(stderrBuf.String()) != "" {
			err = fmt.Errorf("%w (%s)", err, strings.TrimSpace(stderrBuf.String()))
		}
		waitCh <- err
		close(waitCh)
	}()

	reader := bufio.NewReader(stdout)
	metaLine, err := reader.ReadString('\n')
	if err != nil {
		cancel()
		_ = stdout.Close()
		waitErr := <-waitCh
		if waitErr != nil {
			return nil, fmt.Errorf("read messages sidecar stream header: %w", waitErr)
		}
		return nil, fmt.Errorf("read messages sidecar stream header: %w", err)
	}

	var decoded messagesNodeSidecarResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(metaLine)), &decoded); err != nil {
		cancel()
		_ = stdout.Close()
		_ = <-waitCh
		return nil, fmt.Errorf("decode messages sidecar stream header: %w", err)
	}
	if decoded.Error != "" {
		cancel()
		_ = stdout.Close()
		_ = <-waitCh
		return nil, errors.New(decoded.Error)
	}
	if decoded.Status <= 0 {
		cancel()
		_ = stdout.Close()
		_ = <-waitCh
		return nil, errors.New("invalid sidecar stream response status")
	}

	resp := &http.Response{
		StatusCode:    decoded.Status,
		Header:        make(http.Header),
		ContentLength: -1,
		Body: &messagesNodeSidecarStreamBody{
			reader: reader,
			stdout: stdout,
			cancel: cancel,
			waitCh: waitCh,
		},
		Request: req,
	}
	for key, values := range decoded.Headers {
		for _, value := range values {
			resp.Header.Add(key, value)
		}
	}
	return resp, nil
}
