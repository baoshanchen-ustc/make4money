// backend/internal/handler/copilot_body_size_test.go
package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// stubPlatformConfigQuerier 实现 copilotPlatformConfigQuerier 接口，供测试使用。
type stubPlatformConfigQuerier struct {
	entry *service.CopilotPlatformConfigEntry
	err   error
}

func (s *stubPlatformConfigQuerier) GetByPlanType(_ context.Context, _ string) (*service.CopilotPlatformConfigEntry, error) {
	return s.entry, s.err
}

// TestCopilotGatewayHandler_platformBodyLimit_HitsPlatformConfig 验证平台配置命中时返回 max_body_kb * 1024。
func TestCopilotGatewayHandler_platformBodyLimit_HitsPlatformConfig(t *testing.T) {
	kb := 512
	h := &CopilotGatewayHandler{
		defaultMaxBodyBytes: 256 * 1024,
		platformConfigSvc: &stubPlatformConfigQuerier{
			entry: &service.CopilotPlatformConfigEntry{
				PlanType:  "business",
				MaxBodyKB: &kb,
			},
		},
	}
	account := &service.Account{
		Platform:    service.PlatformCopilot,
		Credentials: map[string]any{"plan_type": "business"},
	}
	limit := h.platformBodyLimit(context.Background(), account)
	if limit != 512*1024 {
		t.Errorf("expected platform config 524288 bytes, got %d", limit)
	}
}

// TestCopilotGatewayHandler_platformBodyLimit_FallsBackToDefault 验证
// platformConfigSvc 为 nil 时使用系统默认。
func TestCopilotGatewayHandler_platformBodyLimit_FallsBackToDefault(t *testing.T) {
	h := &CopilotGatewayHandler{
		defaultMaxBodyBytes: 128 * 1024,
		platformConfigSvc:   nil,
	}
	limit := h.platformBodyLimit(context.Background(), nil)
	if limit != 128*1024 {
		t.Errorf("expected system default 131072, got %d", limit)
	}
}

// TestCopilotGatewayHandler_platformBodyLimit_NilMaxBodyKB 验证
// 平台配置存在但 MaxBodyKB 为 nil 时 fallback 系统默认。
func TestCopilotGatewayHandler_platformBodyLimit_NilMaxBodyKB(t *testing.T) {
	h := &CopilotGatewayHandler{
		defaultMaxBodyBytes: 64 * 1024,
		platformConfigSvc: &stubPlatformConfigQuerier{
			entry: &service.CopilotPlatformConfigEntry{
				PlanType:  "individual_free",
				MaxBodyKB: nil,
			},
		},
	}
	account := &service.Account{
		Platform:    service.PlatformCopilot,
		Credentials: map[string]any{"plan_type": "individual_free"},
	}
	limit := h.platformBodyLimit(context.Background(), account)
	if limit != 64*1024 {
		t.Errorf("expected system default 65536, got %d", limit)
	}
}
