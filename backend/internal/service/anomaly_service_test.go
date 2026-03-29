package service

import (
	"context"
	"fmt"
	"testing"
)

func TestDetectAnomalies_ZeroToken(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
	}
	types := detectAnomalies(0, 0, 5000, 200, settings)
	if len(types) != 1 || types[0] != AnomalyZeroToken {
		t.Errorf("expected [zero_token], got %v", types)
	}
}

func TestDetectAnomalies_SlowRequest(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
	}
	types := detectAnomalies(100, 200, 25000, 200, settings)
	if len(types) != 1 || types[0] != AnomalySlowRequest {
		t.Errorf("expected [slow_request], got %v", types)
	}
}

func TestDetectAnomalies_Timeout(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
	}
	types := detectAnomalies(0, 0, 70000, 200, settings)
	found := map[string]bool{}
	for _, at := range types {
		found[string(at)] = true
	}
	if !found["zero_token"] || !found["timeout"] || found["slow_request"] {
		t.Errorf("expected [zero_token, timeout], got %v", types)
	}
}

func TestDetectAnomalies_Error(t *testing.T) {
	settings := &AnomalySettings{SlowRequestThresholdMs: 20000, TimeoutThresholdMs: 60000}
	types := detectAnomalies(100, 200, 1000, 500, settings)
	if len(types) != 1 || types[0] != AnomalyError {
		t.Errorf("expected [error], got %v", types)
	}
}

func TestDetectAnomalies_Normal(t *testing.T) {
	settings := &AnomalySettings{SlowRequestThresholdMs: 20000, TimeoutThresholdMs: 60000, DetectZeroToken: true}
	types := detectAnomalies(100, 200, 5000, 200, settings)
	if len(types) != 0 {
		t.Errorf("expected no anomalies, got %v", types)
	}
}

func TestDetectAnomalies_ZeroTokenDisabled(t *testing.T) {
	settings := &AnomalySettings{SlowRequestThresholdMs: 20000, TimeoutThresholdMs: 60000, DetectZeroToken: false}
	types := detectAnomalies(0, 0, 5000, 200, settings)
	if len(types) != 0 {
		t.Errorf("expected no anomalies when detect_zero_token=false, got %v", types)
	}
}

func TestDetectAnomalies_ExactSlowThreshold(t *testing.T) {
	settings := &AnomalySettings{
		SlowRequestThresholdMs: 20000,
		TimeoutThresholdMs:     60000,
		DetectZeroToken:        true,
	}
	// Exactly at threshold should NOT trigger (strictly greater-than)
	types := detectAnomalies(100, 200, 20000, 200, settings)
	if len(types) != 0 {
		t.Errorf("duration exactly at slow threshold should not trigger, got %v", types)
	}
	// One ms over should trigger
	types = detectAnomalies(100, 200, 20001, 200, settings)
	if len(types) != 1 || types[0] != AnomalySlowRequest {
		t.Errorf("duration 1ms over threshold should trigger slow_request, got %v", types)
	}
}

// fakeSettingRepo is a minimal stub for testing AnomalyService cache.
type fakeSettingRepo struct {
	vals   map[string]string
	getErr error
	calls  int
}

func (f *fakeSettingRepo) Get(ctx context.Context, key string) (*Setting, error) {
	v, ok := f.vals[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: v}, nil
}
func (f *fakeSettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	return f.vals[key], f.getErr
}
func (f *fakeSettingRepo) Set(ctx context.Context, key, val string) error { return nil }
func (f *fakeSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	f.calls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	result := make(map[string]string)
	for _, k := range keys {
		if v, ok := f.vals[k]; ok {
			result[k] = v
		}
	}
	return result, nil
}
func (f *fakeSettingRepo) SetMultiple(ctx context.Context, kvs map[string]string) error { return nil }
func (f *fakeSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	return f.vals, f.getErr
}
func (f *fakeSettingRepo) Delete(ctx context.Context, key string) error { return nil }

func TestAnomalyService_GetSettings_CacheHit(t *testing.T) {
	repo := &fakeSettingRepo{
		vals: map[string]string{
			"ops.anomaly.slow_request_threshold_ms": "15000",
			"ops.anomaly.timeout_threshold_ms":      "45000",
			"ops.anomaly.detect_zero_token":         "true",
			"ops.anomaly.save_raw_data":             "true",
		},
	}
	svc := NewAnomalyService(repo, nil)

	ctx := context.Background()
	s1 := svc.GetSettings(ctx)
	s2 := svc.GetSettings(ctx)

	// Second call should use cache — only 1 DB call total
	if repo.calls != 1 {
		t.Errorf("expected 1 DB call (cache hit on second), got %d", repo.calls)
	}
	if s1.SlowRequestThresholdMs != 15000 {
		t.Errorf("expected SlowRequestThresholdMs=15000, got %d", s1.SlowRequestThresholdMs)
	}
	if s2.SlowRequestThresholdMs != 15000 {
		t.Errorf("cache returned different value")
	}
}

func TestAnomalyService_GetSettings_DBError_UsesDefaults(t *testing.T) {
	repo := &fakeSettingRepo{getErr: fmt.Errorf("db error")}
	svc := NewAnomalyService(repo, nil)

	s := svc.GetSettings(context.Background())
	if s.SlowRequestThresholdMs != 20000 {
		t.Errorf("expected default SlowRequestThresholdMs=20000, got %d", s.SlowRequestThresholdMs)
	}
}
