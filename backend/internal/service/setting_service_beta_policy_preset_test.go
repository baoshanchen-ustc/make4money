//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// betaPolicyRepoStub 是用于 BetaPolicy preset 的最小 settings 仓库 stub。
// 仅响应 GetValue / Set，其它方法直接 panic 以暴露非预期调用。
type betaPolicyRepoStub struct {
	values     map[string]string
	setCalls   map[string]string // 记录 SetValue 调用，用于断言写入
	getErr     error
	getNotFnd  bool
	setError   error
}

func newBetaPolicyRepoStub() *betaPolicyRepoStub {
	return &betaPolicyRepoStub{
		values:   map[string]string{},
		setCalls: map[string]string{},
	}
}

func (s *betaPolicyRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *betaPolicyRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	if s.getNotFnd {
		return "", ErrSettingNotFound
	}
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *betaPolicyRepoStub) Set(ctx context.Context, key, value string) error {
	if s.setError != nil {
		return s.setError
	}
	s.values[key] = value
	s.setCalls[key] = value
	return nil
}

func (s *betaPolicyRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *betaPolicyRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *betaPolicyRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *betaPolicyRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

// TestDefaultBetaPolicySettings_PresetIsConservative 默认配置的 preset 必须为 conservative。
func TestDefaultBetaPolicySettings_PresetIsConservative(t *testing.T) {
	got := DefaultBetaPolicySettings()
	if got.Preset != BetaPolicyPresetConservative {
		t.Fatalf("DefaultBetaPolicySettings().Preset = %q, want %q", got.Preset, BetaPolicyPresetConservative)
	}
}

// TestGetBetaPolicySettings_ReturnsConservativeWhenSettingMissing
// 仓库报告 setting 不存在时，service 返回默认配置（含 Preset=conservative）。
func TestGetBetaPolicySettings_ReturnsConservativeWhenSettingMissing(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	repo.getNotFnd = true
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetBetaPolicySettings(context.Background())
	if err != nil {
		t.Fatalf("GetBetaPolicySettings() error: %v", err)
	}
	if got.Preset != BetaPolicyPresetConservative {
		t.Fatalf("Preset = %q, want %q", got.Preset, BetaPolicyPresetConservative)
	}
}

// TestGetBetaPolicySettings_OldRowWithoutPresetFallsBackToConservative
// 旧 JSON 缺 preset 字段时，service 在读取层 fallback 到 conservative，且不回写 DB。
func TestGetBetaPolicySettings_OldRowWithoutPresetFallsBackToConservative(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	repo.values[SettingKeyBetaPolicySettings] = `{"rules":[{"beta_token":"foo","action":"filter","scope":"all"}]}`
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetBetaPolicySettings(context.Background())
	if err != nil {
		t.Fatalf("GetBetaPolicySettings() error: %v", err)
	}
	if got.Preset != BetaPolicyPresetConservative {
		t.Fatalf("Preset = %q, want %q", got.Preset, BetaPolicyPresetConservative)
	}
	if len(got.Rules) != 1 || got.Rules[0].BetaToken != "foo" {
		t.Fatalf("rules not preserved through preset fallback: %+v", got.Rules)
	}
	// 关键：fallback 不能回写 DB
	if _, wrote := repo.setCalls[SettingKeyBetaPolicySettings]; wrote {
		t.Fatalf("Get path must not write back; got setCalls=%v", repo.setCalls)
	}
}

// TestGetBetaPolicySettings_ClaudeCodeCompatRoundTrip 合法 preset 正常透传。
func TestGetBetaPolicySettings_ClaudeCodeCompatRoundTrip(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	repo.values[SettingKeyBetaPolicySettings] = `{"preset":"claude_code_compat","rules":[]}`
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetBetaPolicySettings(context.Background())
	if err != nil {
		t.Fatalf("GetBetaPolicySettings() error: %v", err)
	}
	if got.Preset != BetaPolicyPresetClaudeCodeCompat {
		t.Fatalf("Preset = %q, want %q", got.Preset, BetaPolicyPresetClaudeCodeCompat)
	}
}

// TestGetBetaPolicySettings_UnknownPresetFallsBackToConservative
// 持久化值是脏数据时 service 也要 fail-safe 回到 conservative，避免请求路径 panic。
func TestGetBetaPolicySettings_UnknownPresetFallsBackToConservative(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	repo.values[SettingKeyBetaPolicySettings] = `{"preset":"bogus","rules":[]}`
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetBetaPolicySettings(context.Background())
	if err != nil {
		t.Fatalf("GetBetaPolicySettings() error: %v", err)
	}
	if got.Preset != BetaPolicyPresetConservative {
		t.Fatalf("Preset = %q, want %q (dirty data must fall back)", got.Preset, BetaPolicyPresetConservative)
	}
}

// TestSetBetaPolicySettings_EmptyPresetNormalizesToConservative
// 写入空 preset 时持久化 layer 自动归一化为 conservative。
func TestSetBetaPolicySettings_EmptyPresetNormalizesToConservative(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SetBetaPolicySettings(context.Background(), &BetaPolicySettings{
		Preset: "",
		Rules: []BetaPolicyRule{
			{BetaToken: "foo", Action: BetaPolicyActionFilter, Scope: BetaPolicyScopeAll},
		},
	})
	if err != nil {
		t.Fatalf("SetBetaPolicySettings() error: %v", err)
	}

	persisted, ok := repo.setCalls[SettingKeyBetaPolicySettings]
	if !ok {
		t.Fatal("expected Set to be called for beta_policy_settings")
	}
	if !contains(persisted, `"preset":"conservative"`) {
		t.Fatalf("persisted JSON should normalize empty preset to conservative; got %s", persisted)
	}
}

// TestSetBetaPolicySettings_RejectsUnknownPreset 拒绝未识别 preset，避免管理员误录入。
func TestSetBetaPolicySettings_RejectsUnknownPreset(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SetBetaPolicySettings(context.Background(), &BetaPolicySettings{
		Preset: "bogus",
		Rules: []BetaPolicyRule{
			{BetaToken: "foo", Action: BetaPolicyActionFilter, Scope: BetaPolicyScopeAll},
		},
	})
	if err == nil {
		t.Fatal("SetBetaPolicySettings() with unknown preset should return error")
	}
	if !contains(err.Error(), "preset") {
		t.Fatalf("error message should mention preset; got: %v", err)
	}
	if _, wrote := repo.setCalls[SettingKeyBetaPolicySettings]; wrote {
		t.Fatal("must not persist when validation fails")
	}
}

// TestSetBetaPolicySettings_ClaudeCodeCompatPersists 合法 preset 直接持久化。
func TestSetBetaPolicySettings_ClaudeCodeCompatPersists(t *testing.T) {
	repo := newBetaPolicyRepoStub()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SetBetaPolicySettings(context.Background(), &BetaPolicySettings{
		Preset: BetaPolicyPresetClaudeCodeCompat,
		Rules:  []BetaPolicyRule{},
	})
	if err != nil {
		t.Fatalf("SetBetaPolicySettings() error: %v", err)
	}
	persisted := repo.setCalls[SettingKeyBetaPolicySettings]
	if !contains(persisted, `"preset":"claude_code_compat"`) {
		t.Fatalf("expected claude_code_compat in persisted JSON; got %s", persisted)
	}
}

// TestIsValidBetaPolicyPreset 边界值。
func TestIsValidBetaPolicyPreset(t *testing.T) {
	cases := map[string]bool{
		"":                                 false, // 空字符串视为未指定，由调用方决定 fallback
		BetaPolicyPresetConservative:       true,
		BetaPolicyPresetClaudeCodeCompat:   true,
		"CONSERVATIVE":                     false, // case-sensitive
		"claude_code_compat ":              false, // trailing whitespace not accepted
		"unknown_preset":                   false,
	}
	for in, want := range cases {
		got := IsValidBetaPolicyPreset(in)
		if got != want {
			t.Errorf("IsValidBetaPolicyPreset(%q) = %v, want %v", in, got, want)
		}
	}
}

// contains 简单 substring helper（避免在 unit 测试里引入完整 strings 包依赖到测试文件外的代码）。
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
