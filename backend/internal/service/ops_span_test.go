package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestAppendOpsSpan_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	now := time.Now()
	service.AppendOpsSpan(c, service.OpsSpan{
		Name:        "auth.verify",
		StartUnixMs: now.UnixMilli(),
		DurationMs:  12,
		Status:      "ok",
	})

	spans := service.GetOpsSpans(c)
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "auth.verify" {
		t.Errorf("expected name auth.verify, got %s", spans[0].Name)
	}
}

func TestMarshalOpsSpans_EmptyIsNil(t *testing.T) {
	result := service.MarshalOpsSpans(nil)
	if result != nil {
		t.Errorf("expected nil for empty spans, got %v", result)
	}
}

func TestMarshalOpsSpans_Valid(t *testing.T) {
	spans := []*service.OpsSpan{
		{Name: "routing.select_account", StartUnixMs: 1000, DurationMs: 5, Status: "ok"},
	}
	result := service.MarshalOpsSpans(spans)
	if result == nil {
		t.Fatal("expected non-nil JSON string")
	}

	var parsed []*service.OpsSpan
	if err := json.Unmarshal([]byte(*result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Name != "routing.select_account" {
		t.Errorf("parsed span mismatch: %+v", parsed)
	}
}

func TestOpsSpan_EndSetsFields(t *testing.T) {
	span := service.NewOpsSpan("token.fetch")
	// Small sleep to ensure some time elapses
	time.Sleep(1 * time.Millisecond)
	span.End("ok")

	if span.DurationMs <= 0 {
		t.Errorf("expected DurationMs > 0, got %d", span.DurationMs)
	}
	if span.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", span.Status)
	}
}

func TestAppendOpsSpan_Accumulates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	service.AppendOpsSpan(c, service.OpsSpan{Name: "routing.select", DurationMs: 5})
	service.AppendOpsSpan(c, service.OpsSpan{Name: "token.fetch", DurationMs: 10})

	spans := service.GetOpsSpans(c)
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Name != "routing.select" {
		t.Errorf("expected first span 'routing.select', got %q", spans[0].Name)
	}
	if spans[1].Name != "token.fetch" {
		t.Errorf("expected second span 'token.fetch', got %q", spans[1].Name)
	}
}
