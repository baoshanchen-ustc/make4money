//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// TestClaudeCodeCompatAllowedBetas_Sentinel 防止维护者悄悄改动允许列表后没更新文档。
// 该列表是产品策略，每次扩缩容都应同步维护文档（见 hook/docs/sub2api-claude-code-maintenance-notes.md）。
func TestClaudeCodeCompatAllowedBetas_Sentinel(t *testing.T) {
	expected := map[string]struct{}{
		"fast-mode-2026-02-01":  {},
		"context-1m-2025-08-07": {},
	}
	if len(claudeCodeCompatAllowedBetas) != len(expected) {
		t.Fatalf("claudeCodeCompatAllowedBetas size changed from %d to %d; update sentinel + docs",
			len(expected), len(claudeCodeCompatAllowedBetas))
	}
	for token := range expected {
		if _, ok := claudeCodeCompatAllowedBetas[token]; !ok {
			t.Errorf("expected %q in claudeCodeCompatAllowedBetas; got %v", token, claudeCodeCompatAllowedBetas)
		}
	}
}

// gatewayBetaPolicyTestRepo 是用于 T8 集成测试的最小 settings 仓库 stub。
type gatewayBetaPolicyTestRepo struct{ raw string }

func (r *gatewayBetaPolicyTestRepo) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get")
}
func (r *gatewayBetaPolicyTestRepo) GetValue(ctx context.Context, key string) (string, error) {
	if r.raw == "" {
		return "", ErrSettingNotFound
	}
	return r.raw, nil
}
func (r *gatewayBetaPolicyTestRepo) Set(ctx context.Context, key, value string) error {
	r.raw = value
	return nil
}
func (r *gatewayBetaPolicyTestRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple")
}
func (r *gatewayBetaPolicyTestRepo) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple")
}
func (r *gatewayBetaPolicyTestRepo) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll")
}
func (r *gatewayBetaPolicyTestRepo) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete")
}

func newGatewayWithBetaPolicy(rawJSON string) *GatewayService {
	repo := &gatewayBetaPolicyTestRepo{raw: rawJSON}
	settingSvc := NewSettingService(repo, &config.Config{})
	return &GatewayService{settingService: settingSvc}
}

func newOAuthAccount() *Account {
	return &Account{Type: AccountTypeOAuth}
}

// TestEvaluateBetaPolicy_ConservativePresetFiltersDefaults
// 保守预设保持历史行为：default rules 中的 fast-mode 与 context-1m 都被 filter。
func TestEvaluateBetaPolicy_ConservativePresetFiltersDefaults(t *testing.T) {
	// 模拟空 settings → service 返回 DefaultBetaPolicySettings()（preset=conservative）
	svc := newGatewayWithBetaPolicy("")
	result := svc.evaluateBetaPolicy(context.Background(), "", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if _, ok := result.filterSet["fast-mode-2026-02-01"]; !ok {
		t.Errorf("conservative preset should filter fast-mode-2026-02-01; got %v", result.filterSet)
	}
	if _, ok := result.filterSet["context-1m-2025-08-07"]; !ok {
		t.Errorf("conservative preset should filter context-1m-2025-08-07; got %v", result.filterSet)
	}
}

// TestEvaluateBetaPolicy_CompatPresetSkipsAllowedFilters
// 兼容预设：即使 rules 列表里仍写着 Filter fast-mode 和 context-1m，也要让它们透传。
func TestEvaluateBetaPolicy_CompatPresetSkipsAllowedFilters(t *testing.T) {
	raw := `{
		"preset": "claude_code_compat",
		"rules": [
			{"beta_token":"fast-mode-2026-02-01","action":"filter","scope":"all"},
			{"beta_token":"context-1m-2025-08-07","action":"filter","scope":"all"}
		]
	}`
	svc := newGatewayWithBetaPolicy(raw)
	result := svc.evaluateBetaPolicy(context.Background(), "", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if _, ok := result.filterSet["fast-mode-2026-02-01"]; ok {
		t.Errorf("compat preset should skip fast-mode-2026-02-01 filter; got filterSet=%v", result.filterSet)
	}
	if _, ok := result.filterSet["context-1m-2025-08-07"]; ok {
		t.Errorf("compat preset should skip context-1m-2025-08-07 filter; got filterSet=%v", result.filterSet)
	}
}

// TestEvaluateBetaPolicy_CompatPresetDoesNotAffectOtherTokens
// 兼容预设只跳过白名单内的 token；其它 Filter 规则继续生效。
func TestEvaluateBetaPolicy_CompatPresetDoesNotAffectOtherTokens(t *testing.T) {
	raw := `{
		"preset": "claude_code_compat",
		"rules": [
			{"beta_token":"fast-mode-2026-02-01","action":"filter","scope":"all"},
			{"beta_token":"some-other-beta","action":"filter","scope":"all"}
		]
	}`
	svc := newGatewayWithBetaPolicy(raw)
	result := svc.evaluateBetaPolicy(context.Background(), "", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if _, ok := result.filterSet["fast-mode-2026-02-01"]; ok {
		t.Errorf("fast-mode should be skipped under compat preset")
	}
	if _, ok := result.filterSet["some-other-beta"]; !ok {
		t.Errorf("non-allowlisted Filter rule should still apply; got %v", result.filterSet)
	}
}

// TestEvaluateBetaPolicy_CompatPresetSkipsBlockToo
// 兼容预设也跳过 Block 规则（避免管理员忘了清理 block rule 后预设被吞）。
func TestEvaluateBetaPolicy_CompatPresetSkipsBlockToo(t *testing.T) {
	raw := `{
		"preset": "claude_code_compat",
		"rules": [
			{"beta_token":"context-1m-2025-08-07","action":"block","scope":"all","error_message":"banned"}
		]
	}`
	svc := newGatewayWithBetaPolicy(raw)
	result := svc.evaluateBetaPolicy(context.Background(), "context-1m-2025-08-07", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if result.blockErr != nil {
		t.Errorf("compat preset should skip block for allowlisted token; got block err: %v", result.blockErr)
	}
}

// TestEvaluateBetaPolicy_ConservativePresetEnforcesBlock
// 保守预设下 Block 规则正常生效。
func TestEvaluateBetaPolicy_ConservativePresetEnforcesBlock(t *testing.T) {
	raw := `{
		"preset": "conservative",
		"rules": [
			{"beta_token":"context-1m-2025-08-07","action":"block","scope":"all","error_message":"banned"}
		]
	}`
	svc := newGatewayWithBetaPolicy(raw)
	result := svc.evaluateBetaPolicy(context.Background(), "context-1m-2025-08-07", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if result.blockErr == nil {
		t.Fatal("conservative preset should still block when rule is Block + token present in header")
	}
	if result.blockErr.Message != "banned" {
		t.Errorf("expected blockErr.Message=%q, got %q", "banned", result.blockErr.Message)
	}
}

// TestEvaluateBetaPolicy_OldDataWithoutPresetActsConservatively
// 旧 settings 缺 preset 字段时按 conservative 处理（service-layer fallback）。
func TestEvaluateBetaPolicy_OldDataWithoutPresetActsConservatively(t *testing.T) {
	raw := `{
		"rules": [
			{"beta_token":"fast-mode-2026-02-01","action":"filter","scope":"all"}
		]
	}`
	svc := newGatewayWithBetaPolicy(raw)
	result := svc.evaluateBetaPolicy(context.Background(), "", newOAuthAccount(), "claude-sonnet-4-5-20250929")

	if _, ok := result.filterSet["fast-mode-2026-02-01"]; !ok {
		t.Errorf("missing preset should default to conservative and apply Filter; got %v", result.filterSet)
	}
}
