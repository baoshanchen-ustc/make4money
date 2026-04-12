package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// TestCopilotPlatformConfigHandler_SignatureCheck 验证 Handler 方法签名存在。
func TestCopilotPlatformConfigHandler_SignatureCheck(t *testing.T) {
	h := &CopilotPlatformConfigHandler{}
	_ = h.List
	_ = h.Update
}

// TestEntryToConfigResponse_NilToEmpty 验证 nil slice/map 被转换为空值而非 null。
func TestEntryToConfigResponse_NilToEmpty(t *testing.T) {
	e := service.CopilotPlatformConfigEntry{
		PlanType:       "individual_free",
		ModelMapping:   nil,
		ModelWhitelist: nil,
	}
	resp := entryToConfigResponse(e)
	if resp.ModelMapping == nil {
		t.Error("expected ModelMapping to be non-nil empty map, got nil")
	}
	if resp.ModelWhitelist == nil {
		t.Error("expected ModelWhitelist to be non-nil empty slice, got nil")
	}
}

// TestIsValidCopilotPlanType 验证合法和非法 plan_type 的判断。
func TestIsValidCopilotPlanType(t *testing.T) {
	valid := []string{"individual_free", "individual_pro", "individual_pro_plus", "business", "enterprise"}
	for _, pt := range valid {
		if !isValidCopilotPlanType(pt) {
			t.Errorf("expected %q to be valid", pt)
		}
	}
	if isValidCopilotPlanType("unknown") {
		t.Error("expected 'unknown' to be invalid")
	}
}
