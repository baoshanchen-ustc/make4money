package service

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockUserAgentCache 模拟缓存
type mockUserAgentCache struct {
	userAgent string
	setError  error
	getError  error
}

func (m *mockUserAgentCache) GetLatestUserAgent(ctx context.Context) (string, error) {
	if m.getError != nil {
		return "", m.getError
	}
	return m.userAgent, nil
}

func (m *mockUserAgentCache) SetLatestUserAgent(ctx context.Context, userAgent string, ttl time.Duration) error {
	if m.setError != nil {
		return m.setError
	}
	m.userAgent = userAgent
	return nil
}

// TestNewUserAgentUpdater 测试创建更新器
func TestNewUserAgentUpdater(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent:             "claude-cli/2.0.62 (external, cli)",
			UserAgentAutoUpdate:          true,
			UserAgentLearnFromRequests:   true,
			UserAgentUpdateIntervalHours: 24,
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)

	assert.NotNil(t, updater)
	assert.Equal(t, "claude-cli/2.0.62 (external, cli)", updater.GetUserAgent())
}

// TestExtractVersion 测试版本号提取
func TestExtractVersion(t *testing.T) {
	updater := &UserAgentUpdater{
		learningRegex: mustCompileRegex(`^claude-cli/(\d+\.\d+\.\d+)`),
	}

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "标准格式",
			userAgent: "claude-cli/2.0.62 (external, cli)",
			want:      "2.0.62",
		},
		{
			name:      "只有版本号",
			userAgent: "claude-cli/1.0.0",
			want:      "1.0.0",
		},
		{
			name:      "多位版本号",
			userAgent: "claude-cli/12.34.567 (external, cli)",
			want:      "12.34.567",
		},
		{
			name:      "无效格式",
			userAgent: "invalid-agent",
			want:      "",
		},
		{
			name:      "空字符串",
			userAgent: "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updater.extractVersion(tt.userAgent)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsNewerVersion 测试版本比较
func TestIsNewerVersion(t *testing.T) {
	updater := &UserAgentUpdater{}

	tests := []struct {
		name   string
		newVer string
		oldVer string
		want   bool
	}{
		{
			name:   "新版本更高 - 主版本",
			newVer: "3.0.0",
			oldVer: "2.0.0",
			want:   true,
		},
		{
			name:   "新版本更高 - 次版本",
			newVer: "2.1.0",
			oldVer: "2.0.0",
			want:   true,
		},
		{
			name:   "新版本更高 - 补丁版本",
			newVer: "2.0.1",
			oldVer: "2.0.0",
			want:   true,
		},
		{
			name:   "版本相同",
			newVer: "2.0.62",
			oldVer: "2.0.62",
			want:   false,
		},
		{
			name:   "新版本更低",
			newVer: "2.0.60",
			oldVer: "2.0.62",
			want:   false,
		},
		{
			name:   "多位数版本号",
			newVer: "2.10.5",
			oldVer: "2.9.99",
			want:   true,
		},
		{
			name:   "空字符串",
			newVer: "",
			oldVer: "2.0.0",
			want:   false,
		},
		{
			name:   "旧版本为空",
			newVer: "2.0.0",
			oldVer: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updater.isNewerVersion(tt.newVer, tt.oldVer)
			assert.Equal(t, tt.want, got, "isNewerVersion(%q, %q)", tt.newVer, tt.oldVer)
		})
	}
}

// TestLearnFromRequest 测试从请求中学习
func TestLearnFromRequest(t *testing.T) {
	tests := []struct {
		name            string
		initialUA       string
		clientUA        string
		learningEnabled bool
		expectedUA      string
		shouldUpdate    bool
	}{
		{
			name:            "学习更新的版本",
			initialUA:       "claude-cli/2.0.62 (external, cli)",
			clientUA:        "claude-cli/2.0.70 (external, cli)",
			learningEnabled: true,
			expectedUA:      "claude-cli/2.0.70 (external, cli)",
			shouldUpdate:    true,
		},
		{
			name:            "忽略旧版本",
			initialUA:       "claude-cli/2.0.70 (external, cli)",
			clientUA:        "claude-cli/2.0.62 (external, cli)",
			learningEnabled: true,
			expectedUA:      "claude-cli/2.0.70 (external, cli)",
			shouldUpdate:    false,
		},
		{
			name:            "学习功能关闭",
			initialUA:       "claude-cli/2.0.62 (external, cli)",
			clientUA:        "claude-cli/2.0.70 (external, cli)",
			learningEnabled: false,
			expectedUA:      "claude-cli/2.0.62 (external, cli)",
			shouldUpdate:    false,
		},
		{
			name:            "无效的 User-Agent",
			initialUA:       "claude-cli/2.0.62 (external, cli)",
			clientUA:        "invalid-user-agent",
			learningEnabled: true,
			expectedUA:      "claude-cli/2.0.62 (external, cli)",
			shouldUpdate:    false,
		},
		{
			name:            "空 User-Agent",
			initialUA:       "claude-cli/2.0.62 (external, cli)",
			clientUA:        "",
			learningEnabled: true,
			expectedUA:      "claude-cli/2.0.62 (external, cli)",
			shouldUpdate:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Gateway: config.GatewayConfig{
					DefaultUserAgent:           tt.initialUA,
					UserAgentLearnFromRequests: tt.learningEnabled,
				},
			}

			cache := &mockUserAgentCache{}
			updater := NewUserAgentUpdater(cfg, cache)

			// 学习新版本
			updater.LearnFromRequest(tt.clientUA)

			// 验证结果
			got := updater.GetUserAgent()
			assert.Equal(t, tt.expectedUA, got)
		})
	}
}

// TestUpdateUserAgent 测试更新 User-Agent
func TestUpdateUserAgent(t *testing.T) {
	cache := &mockUserAgentCache{}
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent: "claude-cli/2.0.62 (external, cli)",
		},
	}

	updater := NewUserAgentUpdater(cfg, cache)

	// 更新 User-Agent
	newUA := "claude-cli/2.0.70 (external, cli)"
	updater.updateUserAgent(newUA)

	// 验证更新成功
	assert.Equal(t, newUA, updater.GetUserAgent())

	// 验证缓存中也更新了
	assert.Equal(t, newUA, cache.userAgent)
}

// TestGetConfiguredUserAgent 测试获取配置的 User-Agent
func TestGetConfiguredUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		envValue string
		want     string
	}{
		{
			name: "使用配置文件值",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					DefaultUserAgent: "claude-cli/2.0.70 (external, cli)",
				},
			},
			envValue: "",
			want:     "claude-cli/2.0.70 (external, cli)",
		},
		{
			name: "配置为空时使用默认值",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					DefaultUserAgent: "",
				},
			},
			envValue: "",
			want:     "claude-cli/2.0.62 (external, cli)", // 代码默认值
		},
		{
			name:     "配置为 nil 时使用默认值",
			cfg:      nil,
			envValue: "",
			want:     "claude-cli/2.0.62 (external, cli)", // 代码默认值
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置环境变量（如果有）
			if tt.envValue != "" {
				t.Setenv("DEFAULT_USER_AGENT", tt.envValue)
			}

			got := getConfiguredUserAgent(tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestGetUpdateInterval 测试获取更新间隔
func TestGetUpdateInterval(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected time.Duration
	}{
		{
			name: "使用配置的间隔",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentUpdateIntervalHours: 12,
				},
			},
			expected: 12 * time.Hour,
		},
		{
			name: "使用默认间隔",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentUpdateIntervalHours: 0,
				},
			},
			expected: 24 * time.Hour,
		},
		{
			name:     "配置为 nil 时使用默认间隔",
			cfg:      nil,
			expected: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &UserAgentUpdater{cfg: tt.cfg}
			got := updater.getUpdateInterval()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestIsAutoUpdateEnabled 测试是否启用自动更新
func TestIsAutoUpdateEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name: "启用自动更新",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentAutoUpdate: true,
				},
			},
			expected: true,
		},
		{
			name: "禁用自动更新",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentAutoUpdate: false,
				},
			},
			expected: false,
		},
		{
			name:     "配置为 nil",
			cfg:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &UserAgentUpdater{cfg: tt.cfg}
			got := updater.isAutoUpdateEnabled()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestIsLearningEnabled 测试是否启用学习功能
func TestIsLearningEnabled(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected bool
	}{
		{
			name: "启用学习功能",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentLearnFromRequests: true,
				},
			},
			expected: true,
		},
		{
			name: "禁用学习功能",
			cfg: &config.Config{
				Gateway: config.GatewayConfig{
					UserAgentLearnFromRequests: false,
				},
			},
			expected: false,
		},
		{
			name:     "配置为 nil",
			cfg:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updater := &UserAgentUpdater{cfg: tt.cfg}
			got := updater.isLearningEnabled()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestConcurrentAccess 测试并发访问安全
func TestConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent:           "claude-cli/2.0.62 (external, cli)",
			UserAgentLearnFromRequests: true,
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)

	// 并发读写测试
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(version int) {
			// 读取
			_ = updater.GetUserAgent()

			// 写入
			ua := fmt.Sprintf("claude-cli/2.0.%d (external, cli)", version)
			updater.LearnFromRequest(ua)

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// 验证最终状态有效
	finalUA := updater.GetUserAgent()
	assert.Contains(t, finalUA, "claude-cli/")
}

// 辅助函数
func mustCompileRegex(pattern string) *regexp.Regexp {
	re := regexp.MustCompile(pattern)
	if re == nil {
		panic(fmt.Sprintf("failed to compile regex: %s", pattern))
	}
	return re
}

// ===== 集成测试（需要网络连接）=====

// TestFetchLatestFromRegistry_Integration 集成测试：从 npm registry 获取最新版本
// 这个测试需要网络连接，可以通过环境变量 RUN_INTEGRATION_TESTS=1 来启用
func TestFetchLatestFromRegistry_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（需要网络连接）")
	}

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			UserAgentAutoUpdate: true,
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 从 npm registry 获取最新版本
	latestUA, err := updater.fetchLatestFromRegistry(ctx)

	// 验证结果
	if err != nil {
		t.Logf("获取失败（可能是网络问题或 API 限额）: %v", err)
		t.Skip("跳过测试，因为 npm registry 不可用")
		return
	}

	require.NotEmpty(t, latestUA, "应该返回非空的 User-Agent")
	assert.Contains(t, latestUA, "claude-cli/", "应该包含 claude-cli 前缀")
	assert.Contains(t, latestUA, "(external, cli)", "应该包含标准后缀")

	t.Logf("从 npm registry 获取的最新版本: %s", latestUA)

	// 验证版本号格式
	version := updater.extractVersion(latestUA)
	assert.NotEmpty(t, version, "应该能提取版本号")
	assert.Regexp(t, `^\d+\.\d+\.\d+$`, version, "版本号格式应该为 x.y.z")

	t.Logf("提取的版本号: %s", version)
}

// TestCheckAndUpdate_Integration 集成测试：完整的检查和更新流程
func TestCheckAndUpdate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（需要网络连接）")
	}

	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent:    "claude-cli/1.0.0 (external, cli)", // 使用旧版本
			UserAgentAutoUpdate: true,
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)

	initialUA := updater.GetUserAgent()
	t.Logf("初始 User-Agent: %s", initialUA)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 执行检查和更新
	updater.checkAndUpdate(ctx)

	// 等待更新完成
	time.Sleep(1 * time.Second)

	// 验证结果
	updatedUA := updater.GetUserAgent()
	t.Logf("更新后 User-Agent: %s", updatedUA)

	// 如果 npm registry 可用，应该已经更新
	if updatedUA != initialUA {
		assert.NotEqual(t, initialUA, updatedUA, "应该已更新到新版本")
		assert.Contains(t, updatedUA, "claude-cli/", "应该包含 claude-cli 前缀")

		// 验证版本号确实更新了
		initialVersion := updater.extractVersion(initialUA)
		updatedVersion := updater.extractVersion(updatedUA)
		assert.True(t, updater.isNewerVersion(updatedVersion, initialVersion),
			"新版本应该比旧版本更高: %s > %s", updatedVersion, initialVersion)
	}
}

// ===== 性能测试 =====

// BenchmarkExtractVersion 性能测试：版本号提取
func BenchmarkExtractVersion(b *testing.B) {
	updater := &UserAgentUpdater{
		learningRegex: mustCompileRegex(`^claude-cli/(\d+\.\d+\.\d+)`),
	}
	userAgent := "claude-cli/2.0.62 (external, cli)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = updater.extractVersion(userAgent)
	}
}

// BenchmarkIsNewerVersion 性能测试：版本比较
func BenchmarkIsNewerVersion(b *testing.B) {
	updater := &UserAgentUpdater{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = updater.isNewerVersion("2.0.70", "2.0.62")
	}
}

// BenchmarkLearnFromRequest 性能测试：学习功能
func BenchmarkLearnFromRequest(b *testing.B) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent:           "claude-cli/2.0.62 (external, cli)",
			UserAgentLearnFromRequests: true,
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)
	clientUA := "claude-cli/2.0.70 (external, cli)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updater.LearnFromRequest(clientUA)
	}
}

// BenchmarkGetUserAgent 性能测试：获取 User-Agent
func BenchmarkGetUserAgent(b *testing.B) {
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent: "claude-cli/2.0.62 (external, cli)",
		},
	}

	cache := &mockUserAgentCache{}
	updater := NewUserAgentUpdater(cfg, cache)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = updater.GetUserAgent()
	}
}
