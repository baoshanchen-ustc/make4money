package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// mockSettingRepository is a mock implementation of SettingRepository for testing
type mockSettingRepository struct {
	mu       sync.RWMutex
	settings map[string]string
	getErr   error
}

func newMockSettingRepository() *mockSettingRepository {
	return &mockSettingRepository{
		settings: make(map[string]string),
	}
}

func (m *mockSettingRepository) Get(ctx context.Context, key string) (*Setting, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	value, ok := m.settings[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (m *mockSettingRepository) GetValue(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return "", m.getErr
	}
	value, ok := m.settings[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (m *mockSettingRepository) Set(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings[key] = value
	return nil
}

func (m *mockSettingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make(map[string]string)
	for _, key := range keys {
		if value, ok := m.settings[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (m *mockSettingRepository) SetMultiple(ctx context.Context, settings map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range settings {
		m.settings[k] = v
	}
	return nil
}

func (m *mockSettingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make(map[string]string)
	for k, v := range m.settings {
		result[k] = v
	}
	return result, nil
}

func (m *mockSettingRepository) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.settings, key)
	return nil
}

func (m *mockSettingRepository) setValues(settings map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range settings {
		m.settings[k] = v
	}
}

func TestRechargeSettingsCache_CacheHit(t *testing.T) {
	repo := newMockSettingRepository()
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "10.00",
		SettingKeyRechargeMaxAmount:          "1000.00",
		SettingKeyRechargeDefaultAmounts:     "[50,100,200]",
		SettingKeyRechargeOrderExpireMinutes: "30",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	// First call - loads from DB
	settings1, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}
	if settings1.MinAmount != 10.0 {
		t.Errorf("MinAmount = %v, want 10.0", settings1.MinAmount)
	}
	if settings1.MaxAmount != 1000.0 {
		t.Errorf("MaxAmount = %v, want 1000.0", settings1.MaxAmount)
	}
	if settings1.OrderExpireMinutes != 30 {
		t.Errorf("OrderExpireMinutes = %v, want 30", settings1.OrderExpireMinutes)
	}
	if len(settings1.DefaultAmounts) != 3 {
		t.Errorf("DefaultAmounts length = %v, want 3", len(settings1.DefaultAmounts))
	}

	// Modify DB directly (simulate external change)
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount: "20.00",
	})

	// Second call should return cached value (not the updated DB value)
	settings2, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// Should still be 10.0 from cache
	if settings2.MinAmount != 10.0 {
		t.Errorf("Cache hit: MinAmount = %v, want 10.0 (cached)", settings2.MinAmount)
	}
}

func TestRechargeSettingsCache_CacheExpired(t *testing.T) {
	repo := newMockSettingRepository()
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "10.00",
		SettingKeyRechargeMaxAmount:          "1000.00",
		SettingKeyRechargeDefaultAmounts:     "[50,100,200]",
		SettingKeyRechargeOrderExpireMinutes: "30",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	// Override TTL for testing (very short)
	svc.rechargeCacheTTL = 50 * time.Millisecond

	ctx := context.Background()

	// First call - loads from DB
	settings1, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}
	if settings1.MinAmount != 10.0 {
		t.Errorf("MinAmount = %v, want 10.0", settings1.MinAmount)
	}

	// Modify DB
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount: "20.00",
	})

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Next call should reload from DB (cache expired)
	settings3, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// Should now be 20.0 from refreshed cache
	if settings3.MinAmount != 20.0 {
		t.Errorf("Cache expired: MinAmount = %v, want 20.0 (reloaded)", settings3.MinAmount)
	}
}

func TestRechargeSettingsCache_InvalidateOnUpdate(t *testing.T) {
	repo := newMockSettingRepository()
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "10.00",
		SettingKeyRechargeMaxAmount:          "1000.00",
		SettingKeyRechargeDefaultAmounts:     "[50,100,200]",
		SettingKeyRechargeOrderExpireMinutes: "30",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	// First call - loads from DB
	settings1, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}
	if settings1.MinAmount != 10.0 {
		t.Errorf("MinAmount = %v, want 10.0", settings1.MinAmount)
	}

	// Update settings via service (should invalidate cache)
	newSettings := &RechargeSettings{
		MinAmount:          25.0,
		MaxAmount:          2000.0,
		DefaultAmounts:     []float64{100, 200, 500},
		OrderExpireMinutes: 60,
	}
	if err := svc.UpdateRechargeSettings(ctx, newSettings); err != nil {
		t.Fatalf("UpdateRechargeSettings() error: %v", err)
	}

	// Immediate read should return new value (cache was updated)
	settings2, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	if settings2.MinAmount != 25.0 {
		t.Errorf("After update: MinAmount = %v, want 25.0", settings2.MinAmount)
	}
	if settings2.MaxAmount != 2000.0 {
		t.Errorf("After update: MaxAmount = %v, want 2000.0", settings2.MaxAmount)
	}
	if settings2.OrderExpireMinutes != 60 {
		t.Errorf("After update: OrderExpireMinutes = %v, want 60", settings2.OrderExpireMinutes)
	}
}

func TestRechargeSettingsCache_DefaultValues(t *testing.T) {
	repo := newMockSettingRepository()
	// Empty settings - should use defaults

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	settings, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	if settings.MinAmount != DefaultRechargeMinAmount {
		t.Errorf("Default MinAmount = %v, want %v", settings.MinAmount, DefaultRechargeMinAmount)
	}
	if settings.MaxAmount != DefaultRechargeMaxAmount {
		t.Errorf("Default MaxAmount = %v, want %v", settings.MaxAmount, DefaultRechargeMaxAmount)
	}
	if settings.OrderExpireMinutes != DefaultRechargeOrderExpireMinutes {
		t.Errorf("Default OrderExpireMinutes = %v, want %v", settings.OrderExpireMinutes, DefaultRechargeOrderExpireMinutes)
	}
}

func TestRechargeSettingsCache_ReturnsCopy(t *testing.T) {
	repo := newMockSettingRepository()
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "10.00",
		SettingKeyRechargeMaxAmount:          "1000.00",
		SettingKeyRechargeDefaultAmounts:     "[50,100,200]",
		SettingKeyRechargeOrderExpireMinutes: "30",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	// Get settings
	settings1, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// Modify the returned slice
	originalLen := len(settings1.DefaultAmounts)
	settings1.DefaultAmounts = append(settings1.DefaultAmounts, 999.0)

	// Get settings again
	settings2, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// Should not be affected by modification
	if len(settings2.DefaultAmounts) != originalLen {
		t.Errorf("Cache was modified: DefaultAmounts length = %v, want %v", len(settings2.DefaultAmounts), originalLen)
	}
}

func TestRechargeSettingsCache_ConcurrentAccess(t *testing.T) {
	repo := newMockSettingRepository()
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "10.00",
		SettingKeyRechargeMaxAmount:          "1000.00",
		SettingKeyRechargeDefaultAmounts:     "[50,100,200]",
		SettingKeyRechargeOrderExpireMinutes: "30",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			settings, err := svc.GetRechargeSettings(ctx)
			if err != nil {
				errCh <- err
				return
			}
			// Note: MinAmount may change during concurrent writes, so we just verify no error
			_ = settings.MinAmount
		}()
	}

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			newSettings := &RechargeSettings{
				MinAmount:          float64(10 + idx),
				MaxAmount:          2000.0,
				DefaultAmounts:     []float64{100, 200},
				OrderExpireMinutes: 60,
			}
			if err := svc.UpdateRechargeSettings(ctx, newSettings); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

func TestUpdateRechargeSettings_Validation(t *testing.T) {
	repo := newMockSettingRepository()
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	tests := []struct {
		name     string
		settings *RechargeSettings
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid settings",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{50, 100, 200},
				OrderExpireMinutes: 30,
			},
			wantErr: false,
		},
		{
			name: "min_amount <= 0",
			settings: &RechargeSettings{
				MinAmount:          0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{50},
				OrderExpireMinutes: 30,
			},
			wantErr: true,
			errMsg:  "min_amount must be greater than 0",
		},
		{
			name: "max_amount <= 0",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          0,
				DefaultAmounts:     []float64{50},
				OrderExpireMinutes: 30,
			},
			wantErr: true,
			errMsg:  "max_amount must be greater than 0",
		},
		{
			name: "min > max",
			settings: &RechargeSettings{
				MinAmount:          1000.0,
				MaxAmount:          100.0,
				DefaultAmounts:     []float64{},
				OrderExpireMinutes: 30,
			},
			wantErr: true,
			errMsg:  "min_amount must be less than or equal to max_amount",
		},
		{
			name: "expire minutes < 1",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{},
				OrderExpireMinutes: 0,
			},
			wantErr: true,
			errMsg:  "order_expire_minutes must be between 1 and 1440",
		},
		{
			name: "expire minutes > 1440",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{},
				OrderExpireMinutes: 1441,
			},
			wantErr: true,
			errMsg:  "order_expire_minutes must be between 1 and 1440",
		},
		{
			name: "default amount out of range (below min)",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{5},
				OrderExpireMinutes: 30,
			},
			wantErr: true,
			errMsg:  "default amount 5.00 is out of allowed range",
		},
		{
			name: "default amount out of range (above max)",
			settings: &RechargeSettings{
				MinAmount:          10.0,
				MaxAmount:          1000.0,
				DefaultAmounts:     []float64{2000},
				OrderExpireMinutes: 30,
			},
			wantErr: true,
			errMsg:  "default amount 2000.00 is out of allowed range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.UpdateRechargeSettings(ctx, tt.settings)
			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateRechargeSettings() expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("UpdateRechargeSettings() error = %v, want containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("UpdateRechargeSettings() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSettingService_Stop(t *testing.T) {
	repo := newMockSettingRepository()
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
	}

	svc := NewSettingService(repo, cfg)

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not complete within timeout")
	}
}

// Ensure fmt is used (for potential error formatting)
var _ = fmt.Sprintf

// TestRechargeConfigPriority_DatabaseFirst 测试数据库配置优先
func TestRechargeConfigPriority_DatabaseFirst(t *testing.T) {
	repo := newMockSettingRepository()
	// 数据库设置 min_amount = 5.0
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount:          "5.00",
		SettingKeyRechargeMaxAmount:          "500.00",
		SettingKeyRechargeDefaultAmounts:     "[25,50,100]",
		SettingKeyRechargeOrderExpireMinutes: "45",
	})

	// config.yaml 设置不同的值
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
		Recharge: config.RechargeConfig{
			MinAmount:          2.0,
			MaxAmount:          200.0,
			DefaultAmounts:     []float64{10, 20},
			OrderExpireMinutes: 30,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	settings, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// 应该使用数据库的值
	if settings.MinAmount != 5.0 {
		t.Errorf("MinAmount = %v, want 5.0 (from database)", settings.MinAmount)
	}
	if settings.MaxAmount != 500.0 {
		t.Errorf("MaxAmount = %v, want 500.0 (from database)", settings.MaxAmount)
	}
	if settings.OrderExpireMinutes != 45 {
		t.Errorf("OrderExpireMinutes = %v, want 45 (from database)", settings.OrderExpireMinutes)
	}
	if len(settings.DefaultAmounts) != 3 || settings.DefaultAmounts[0] != 25 {
		t.Errorf("DefaultAmounts = %v, want [25,50,100] (from database)", settings.DefaultAmounts)
	}
}

// TestRechargeConfigPriority_FallbackToConfig 测试回退到 config.yaml
func TestRechargeConfigPriority_FallbackToConfig(t *testing.T) {
	repo := newMockSettingRepository()
	// 数据库为空

	// config.yaml 设置值
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
		Recharge: config.RechargeConfig{
			MinAmount:          2.0,
			MaxAmount:          200.0,
			DefaultAmounts:     []float64{10, 20, 50},
			OrderExpireMinutes: 60,
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	settings, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// 应该使用 config.yaml 的值
	if settings.MinAmount != 2.0 {
		t.Errorf("MinAmount = %v, want 2.0 (from config.yaml)", settings.MinAmount)
	}
	if settings.MaxAmount != 200.0 {
		t.Errorf("MaxAmount = %v, want 200.0 (from config.yaml)", settings.MaxAmount)
	}
	if settings.OrderExpireMinutes != 60 {
		t.Errorf("OrderExpireMinutes = %v, want 60 (from config.yaml)", settings.OrderExpireMinutes)
	}
	if len(settings.DefaultAmounts) != 3 || settings.DefaultAmounts[0] != 10 {
		t.Errorf("DefaultAmounts = %v, want [10,20,50] (from config.yaml)", settings.DefaultAmounts)
	}
}

// TestRechargeConfigPriority_FallbackToDefault 测试回退到代码默认值
func TestRechargeConfigPriority_FallbackToDefault(t *testing.T) {
	repo := newMockSettingRepository()
	// 数据库为空

	// config.yaml 也为空（零值）
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
		Recharge: config.RechargeConfig{
			// 全部为零值
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	settings, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// 应该使用代码默认值
	if settings.MinAmount != DefaultRechargeMinAmount {
		t.Errorf("MinAmount = %v, want %v (default)", settings.MinAmount, DefaultRechargeMinAmount)
	}
	if settings.MaxAmount != DefaultRechargeMaxAmount {
		t.Errorf("MaxAmount = %v, want %v (default)", settings.MaxAmount, DefaultRechargeMaxAmount)
	}
	if settings.OrderExpireMinutes != DefaultRechargeOrderExpireMinutes {
		t.Errorf("OrderExpireMinutes = %v, want %v (default)", settings.OrderExpireMinutes, DefaultRechargeOrderExpireMinutes)
	}
}

// TestRechargeConfigPriority_MixedSources 测试混合来源（各字段来自不同层级）
func TestRechargeConfigPriority_MixedSources(t *testing.T) {
	repo := newMockSettingRepository()
	// 只设置 min_amount 在数据库
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount: "15.00",
	})

	// config.yaml 设置 max_amount 和 default_amounts
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
		Recharge: config.RechargeConfig{
			MinAmount:          5.0,   // 会被数据库覆盖
			MaxAmount:          800.0, // 会使用这个
			DefaultAmounts:     []float64{50, 100, 200},
			OrderExpireMinutes: 0, // 零值，会用代码默认值
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	settings, err := svc.GetRechargeSettings(ctx)
	if err != nil {
		t.Fatalf("GetRechargeSettings() error: %v", err)
	}

	// min_amount 来自数据库
	if settings.MinAmount != 15.0 {
		t.Errorf("MinAmount = %v, want 15.0 (from database)", settings.MinAmount)
	}
	// max_amount 来自 config.yaml
	if settings.MaxAmount != 800.0 {
		t.Errorf("MaxAmount = %v, want 800.0 (from config.yaml)", settings.MaxAmount)
	}
	// default_amounts 来自 config.yaml
	if len(settings.DefaultAmounts) != 3 || settings.DefaultAmounts[0] != 50 {
		t.Errorf("DefaultAmounts = %v, want [50,100,200] (from config.yaml)", settings.DefaultAmounts)
	}
	// order_expire_minutes 来自代码默认值（config.yaml 是 0）
	if settings.OrderExpireMinutes != DefaultRechargeOrderExpireMinutes {
		t.Errorf("OrderExpireMinutes = %v, want %v (default)", settings.OrderExpireMinutes, DefaultRechargeOrderExpireMinutes)
	}
}

// TestRechargeConfigSources 测试配置来源追溯功能
func TestRechargeConfigSources(t *testing.T) {
	repo := newMockSettingRepository()
	// 只设置 min_amount 在数据库
	repo.setValues(map[string]string{
		SettingKeyRechargeMinAmount: "15.00",
	})

	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserConcurrency: 3,
			UserBalance:     10.0,
		},
		Recharge: config.RechargeConfig{
			MinAmount:          5.0, // 会被数据库覆盖
			MaxAmount:          800.0,
			DefaultAmounts:     []float64{50, 100},
			OrderExpireMinutes: 0, // 零值
		},
	}

	svc := NewSettingService(repo, cfg)
	defer svc.Stop()

	ctx := context.Background()

	sources, err := svc.GetRechargeConfigSources(ctx)
	if err != nil {
		t.Fatalf("GetRechargeConfigSources() error: %v", err)
	}

	if sources["min_amount"] != "database" {
		t.Errorf("min_amount source = %v, want 'database'", sources["min_amount"])
	}
	if sources["max_amount"] != "config.yaml" {
		t.Errorf("max_amount source = %v, want 'config.yaml'", sources["max_amount"])
	}
	if sources["default_amounts"] != "config.yaml" {
		t.Errorf("default_amounts source = %v, want 'config.yaml'", sources["default_amounts"])
	}
	if sources["order_expire_minutes"] != "default" {
		t.Errorf("order_expire_minutes source = %v, want 'default'", sources["order_expire_minutes"])
	}
}
