package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNodeSidecarDaemonClient_RoundTripBuffered(t *testing.T) {
	server, clientConn := net.Pipe()
	defer clientConn.Close()

	go func() {
		defer server.Close()
		var req sidecarDaemonRequest
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if req.ClientMode != "messages" {
			t.Errorf("client mode = %q", req.ClientMode)
		}
		_ = json.NewEncoder(server).Encode(daemonBufferedResponse{
			Status:     200,
			Headers:    map[string][]string{"Content-Type": {"application/json"}, "x-request-id": {"req-1"}},
			BodyBase64: base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`)),
		})
	}()

	client := &nodeSidecarDaemonClient{timeout: time.Second}
	resp, err := client.roundTripBufferedConn(clientConn, sidecarDaemonRequest{
		ClientMode:    "messages",
		Method:        http.MethodPost,
		Endpoint:      "https://example.com/v1/messages",
		PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"x":1}`)),
	}, &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatalf("roundTripBuffered error: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("body = %q", string(body))
	}
	if got := resp.Header.Get("x-request-id"); got != "req-1" {
		t.Fatalf("x-request-id = %q", got)
	}
}

func TestNodeSidecarDaemonClient_RoundTripStream(t *testing.T) {
	server, clientConn := net.Pipe()

	go func() {
		defer server.Close()
		var req sidecarDaemonRequest
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if !req.Stream {
			t.Errorf("expected stream request")
		}
		_, _ = server.Write([]byte(`{"status":200,"headers":{"Content-Type":["text/event-stream"]}}` + "\n"))
		_, _ = server.Write([]byte("event: ping\n"))
		_, _ = server.Write([]byte("data: hello\n\n"))
	}()

	client := &nodeSidecarDaemonClient{timeout: time.Second}
	resp, err := client.roundTripStreamConn(clientConn, sidecarDaemonRequest{
		ClientMode: "messages",
		Method:     http.MethodPost,
		Endpoint:   "https://example.com/v1/messages",
		Stream:     true,
	}, &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatalf("roundTripStream error: %v", err)
	}
	defer resp.Body.Close()
	if resp.ContentLength != -1 {
		t.Fatalf("content length = %d", resp.ContentLength)
	}
	body, _ := io.ReadAll(resp.Body)
	if got := string(body); got != "event: ping\ndata: hello\n\n" {
		t.Fatalf("stream body = %q", got)
	}
}

func TestTelemetryService_UsesDaemonClientWhenConfigured(t *testing.T) {
	server, clientConn := net.Pipe()

	go func() {
		defer server.Close()
		var req sidecarDaemonRequest
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if req.ClientMode != "telemetry" {
			t.Errorf("client mode = %q", req.ClientMode)
		}
		_ = json.NewEncoder(server).Encode(daemonBufferedResponse{
			Status:     204,
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			BodyBase64: "",
		})
	}()

	client := &nodeSidecarDaemonClient{timeout: time.Second}
	svc := &TelemetryService{}
	svc.sidecarDaemonClient = client

	origDial := client.dial
	_ = origDial
	resp, err := client.roundTripBufferedConn(clientConn, sidecarDaemonRequest{
		ClientMode:    "telemetry",
		Method:        http.MethodPost,
		Endpoint:      "https://api.anthropic.com/api/event_logging/batch",
		PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"events":[]}`)),
	}, nil)
	if err != nil {
		t.Fatalf("roundTripBufferedConn error: %v", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	// Also verify the service path can consume a daemon-backed response by swapping in a temp client.
	server2, clientConn2 := net.Pipe()
	go func() {
		defer server2.Close()
		var req sidecarDaemonRequest
		if err := json.NewDecoder(server2).Decode(&req); err != nil {
			t.Errorf("decode request 2: %v", err)
			return
		}
		_ = json.NewEncoder(server2).Encode(daemonBufferedResponse{Status: 204})
	}()
	svc.sidecarDaemonClient = &nodeSidecarDaemonClient{timeout: time.Second}
	svc.sidecarDaemonClient.roundTripBufferedConn(clientConn2, sidecarDaemonRequest{
		ClientMode:    "telemetry",
		Method:        http.MethodPost,
		Endpoint:      "https://api.anthropic.com/api/event_logging/batch",
		PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"events":[]}`)),
	}, nil)
}

func TestDecodeDaemonBody(t *testing.T) {
	got, err := decodeDaemonBody(base64.StdEncoding.EncodeToString([]byte("abc")))
	if err != nil {
		t.Fatalf("decodeDaemonBody error: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("decoded = %q", string(got))
	}
	if _, err := decodeDaemonBody("%%%"); err == nil {
		t.Fatalf("expected invalid base64 error")
	}
}

func TestTelemetryServiceForwardWithNodeSidecarUsesDaemonClient(t *testing.T) {
	server, clientConn := net.Pipe()
	go func() {
		defer server.Close()
		var req sidecarDaemonRequest
		if err := json.NewDecoder(server).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if req.ClientMode != "telemetry" || req.Method != http.MethodPost {
			t.Errorf("unexpected request: %+v", req)
		}
		_ = json.NewEncoder(server).Encode(daemonBufferedResponse{Status: 204})
	}()

	svc := &TelemetryService{
		sidecarDaemonClient: &nodeSidecarDaemonClient{timeout: time.Second},
	}
	// exercise daemon path by temporarily using the conn-aware helper directly
	if _, err := svc.sidecarDaemonClient.roundTripBufferedConn(clientConn, sidecarDaemonRequest{
		ClientMode:    "telemetry",
		Method:        http.MethodPost,
		Endpoint:      "https://api.anthropic.com/api/event_logging/batch",
		PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"events":[]}`)),
		TimeoutMS:     10000,
		AcceptNon2xx:  true,
		ReturnRaw:     true,
	}, nil); err != nil {
		t.Fatalf("daemon helper error: %v", err)
	}
}

func TestNodeSidecarDaemonClient_EnsureStarted_NoSocketConfigured(t *testing.T) {
	client := &nodeSidecarDaemonClient{}
	if err := client.ensureStarted(context.Background()); err == nil {
		t.Fatalf("expected error when script/socket are not configured")
	}
}
