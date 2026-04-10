//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// stub repository
// ---------------------------------------------------------------------------

// stubModelPricingRepo 是 ModelPricingRepository 的测试桩。
// 通过字段控制每个方法的返回值，可在每个用例中按需配置。
type stubModelPricingRepo struct {
	listResult    []ModelPricingEntry
	listErr       error
	getByIDResult *ModelPricingEntry
	getByIDErr    error
	createResult  *ModelPricingEntry
	createErr     error
	updateResult  *ModelPricingEntry
	updateErr     error
	deleteErr     error

	// 记录调用参数，用于验证传入值是否规范化
	lastCreated *ModelPricingEntry
	lastUpdated *ModelPricingEntry
}

var _ ModelPricingRepository = (*stubModelPricingRepo)(nil)

func (r *stubModelPricingRepo) List(_ context.Context) ([]ModelPricingEntry, error) {
	return r.listResult, r.listErr
}

func (r *stubModelPricingRepo) GetByKey(_ context.Context, _ string) (*ModelPricingEntry, error) {
	return nil, nil
}

func (r *stubModelPricingRepo) GetByID(_ context.Context, _ int64) (*ModelPricingEntry, error) {
	return r.getByIDResult, r.getByIDErr
}

func (r *stubModelPricingRepo) Create(_ context.Context, entry *ModelPricingEntry) (*ModelPricingEntry, error) {
	r.lastCreated = entry
	if r.createErr != nil {
		return nil, r.createErr
	}
	result := *entry
	result.ID = 1
	if r.createResult != nil {
		return r.createResult, nil
	}
	return &result, nil
}

func (r *stubModelPricingRepo) Update(_ context.Context, _ int64, entry *ModelPricingEntry) (*ModelPricingEntry, error) {
	r.lastUpdated = entry
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	result := *entry
	if r.updateResult != nil {
		return r.updateResult, nil
	}
	return &result, nil
}

func (r *stubModelPricingRepo) Delete(_ context.Context, _ int64) error {
	return r.deleteErr
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestModelPricingService(repo *stubModelPricingRepo) *ModelPricingService {
	return NewModelPricingService(repo)
}

func validEnabledEntry(modelKey string) *ModelPricingEntry {
	return &ModelPricingEntry{
		ModelKey:             modelKey,
		InputPricePerMillion: 1.0,
		Enabled:              true,
	}
}

// ---------------------------------------------------------------------------
// model_key 规范化
// ---------------------------------------------------------------------------

func TestCreate_NormalizesModelKeyToLowercase(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("GPT-5.1")
	_, err := svc.Create(context.Background(), entry)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.1", repo.lastCreated.ModelKey)
}

func TestCreate_NormalizesModelKeyTrimsWhitespace(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("  gpt-5.1  ")
	_, err := svc.Create(context.Background(), entry)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.1", repo.lastCreated.ModelKey)
}

func TestUpdate_NormalizesModelKey(t *testing.T) {
	old := &ModelPricingEntry{ID: 1, ModelKey: "gpt-5.1", InputPricePerMillion: 1.0, Enabled: true}
	repo := &stubModelPricingRepo{getByIDResult: old}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("  GPT-5.2  ")
	_, err := svc.Update(context.Background(), 1, entry)
	require.NoError(t, err)
	require.Equal(t, "gpt-5.2", repo.lastUpdated.ModelKey)
}

// ---------------------------------------------------------------------------
// 空 model_key 返回 400
// ---------------------------------------------------------------------------

func TestCreate_BlankModelKeyReturns400(t *testing.T) {
	tests := []struct {
		name     string
		modelKey string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"tabs and spaces", "\t  \t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubModelPricingRepo{}
			svc := newTestModelPricingService(repo)

			entry := validEnabledEntry(tt.modelKey)
			_, err := svc.Create(context.Background(), entry)
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, "MODEL_PRICING_EMPTY_KEY", infraerrors.Reason(err))
			// repo.Create 不应被调用
			require.Nil(t, repo.lastCreated)
		})
	}
}

func TestUpdate_BlankModelKeyReturns400(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("   ")
	_, err := svc.Update(context.Background(), 1, entry)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "MODEL_PRICING_EMPTY_KEY", infraerrors.Reason(err))
}

// ---------------------------------------------------------------------------
// 全 0 且启用返回 400
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// 全 0 且启用：现已允许（支持免费模型）
// ---------------------------------------------------------------------------

func TestCreate_ZeroPriceEnabledAllowed(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	entry := &ModelPricingEntry{
		ModelKey: "gpt-5.1",
		Enabled:  true,
		// 所有价格字段均为 0（零值）：免费模型场景
	}
	_, err := svc.Create(context.Background(), entry)
	require.NoError(t, err)
}

func TestCreate_DisabledWithZeroPriceSucceeds(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	entry := &ModelPricingEntry{
		ModelKey: "gpt-5.1",
		Enabled:  false,
		// 所有价格字段为 0 时，disabled 条目应该允许保存
	}
	_, err := svc.Create(context.Background(), entry)
	require.NoError(t, err)
}

func TestCreate_AnyNonZeroPriceFieldAllowsEnabled(t *testing.T) {
	tests := []struct {
		name  string
		entry ModelPricingEntry
	}{
		{"only input price", ModelPricingEntry{ModelKey: "m", Enabled: true, InputPricePerMillion: 0.01}},
		{"only output price", ModelPricingEntry{ModelKey: "m", Enabled: true, OutputPricePerMillion: 0.01}},
		{"only cache read price", ModelPricingEntry{ModelKey: "m", Enabled: true, CacheReadPricePerMillion: 0.01}},
		{"only cache creation price", ModelPricingEntry{ModelKey: "m", Enabled: true, CacheCreationPricePerMillion: 0.01}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubModelPricingRepo{}
			svc := newTestModelPricingService(repo)
			e := tt.entry
			_, err := svc.Create(context.Background(), &e)
			require.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// 唯一约束冲突透传为 409
// ---------------------------------------------------------------------------

func TestCreate_ConflictErrorPassedThrough(t *testing.T) {
	repo := &stubModelPricingRepo{createErr: ErrModelPricingExists}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("gpt-5.1")
	_, err := svc.Create(context.Background(), entry)
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "MODEL_PRICING_EXISTS", infraerrors.Reason(err))
}

func TestUpdate_ConflictErrorPassedThrough(t *testing.T) {
	old := &ModelPricingEntry{ID: 1, ModelKey: "gpt-5.1", InputPricePerMillion: 1.0, Enabled: true}
	repo := &stubModelPricingRepo{
		getByIDResult: old,
		updateErr:     ErrModelPricingExists,
	}
	svc := newTestModelPricingService(repo)

	entry := validEnabledEntry("gpt-5.2")
	_, err := svc.Update(context.Background(), 1, entry)
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "MODEL_PRICING_EXISTS", infraerrors.Reason(err))
}

// ---------------------------------------------------------------------------
// 改 model_key 时旧缓存清理
// ---------------------------------------------------------------------------

func TestUpdate_RenameModelKeyEvictsOldCacheEntry(t *testing.T) {
	old := &ModelPricingEntry{ID: 1, ModelKey: "gpt-5.1", InputPricePerMillion: 1.0, Enabled: true}
	repo := &stubModelPricingRepo{
		getByIDResult: old,
		updateResult: &ModelPricingEntry{
			ID: 1, ModelKey: "gpt-5.2", InputPricePerMillion: 2.0, Enabled: true,
		},
	}
	svc := newTestModelPricingService(repo)

	// 预先把旧 key 写入缓存
	svc.mu.Lock()
	svc.cache["gpt-5.1"] = old
	svc.mu.Unlock()

	entry := validEnabledEntry("gpt-5.2")
	entry.InputPricePerMillion = 2.0
	_, err := svc.Update(context.Background(), 1, entry)
	require.NoError(t, err)

	// 旧 key 应被从缓存中驱逐
	require.Nil(t, svc.GetCachedPricing("gpt-5.1"), "旧 model_key 应从缓存中清除")
	// 新 key 应写入缓存
	require.NotNil(t, svc.GetCachedPricing("gpt-5.2"), "新 model_key 应写入缓存")
}

func TestUpdate_SameModelKeyDoesNotEvictCache(t *testing.T) {
	old := &ModelPricingEntry{ID: 1, ModelKey: "gpt-5.1", InputPricePerMillion: 1.0, Enabled: true}
	repo := &stubModelPricingRepo{
		getByIDResult: old,
		updateResult: &ModelPricingEntry{
			ID: 1, ModelKey: "gpt-5.1", InputPricePerMillion: 5.0, Enabled: true,
		},
	}
	svc := newTestModelPricingService(repo)

	svc.mu.Lock()
	svc.cache["gpt-5.1"] = old
	svc.mu.Unlock()

	entry := validEnabledEntry("gpt-5.1")
	entry.InputPricePerMillion = 5.0
	_, err := svc.Update(context.Background(), 1, entry)
	require.NoError(t, err)

	// 同 key 更新后缓存仍应存在，且价格已刷新
	pricing := svc.GetCachedPricing("gpt-5.1")
	require.NotNil(t, pricing)
	require.InDelta(t, 5.0e-6, pricing.InputPricePerToken, 1e-12)
}

// ---------------------------------------------------------------------------
// GetCachedPricing 基本行为
// ---------------------------------------------------------------------------

func TestGetCachedPricing_MissReturnsNil(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	require.Nil(t, svc.GetCachedPricing("nonexistent-model"))
}

func TestGetCachedPricing_HitReturnsConvertedPricing(t *testing.T) {
	repo := &stubModelPricingRepo{}
	svc := newTestModelPricingService(repo)

	svc.mu.Lock()
	svc.cache["gpt-5.1"] = &ModelPricingEntry{
		ModelKey:             "gpt-5.1",
		InputPricePerMillion: 1.25,
		Enabled:              true,
	}
	svc.mu.Unlock()

	pricing := svc.GetCachedPricing("gpt-5.1")
	require.NotNil(t, pricing)
	require.InDelta(t, 1.25e-6, pricing.InputPricePerToken, 1e-12)
}

// ---------------------------------------------------------------------------
// LoadCache
// ---------------------------------------------------------------------------

func TestLoadCache_OnlyLoadsEnabledEntries(t *testing.T) {
	repo := &stubModelPricingRepo{
		listResult: []ModelPricingEntry{
			{ModelKey: "gpt-5.1", InputPricePerMillion: 1.25, Enabled: true},
			{ModelKey: "gpt-5.2", InputPricePerMillion: 1.75, Enabled: false},
		},
	}
	svc := newTestModelPricingService(repo)

	err := svc.LoadCache(context.Background())
	require.NoError(t, err)

	require.NotNil(t, svc.GetCachedPricing("gpt-5.1"), "enabled 条目应写入缓存")
	require.Nil(t, svc.GetCachedPricing("gpt-5.2"), "disabled 条目不应写入缓存")
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete_EvictsCacheEntry(t *testing.T) {
	target := &ModelPricingEntry{ID: 1, ModelKey: "gpt-5.1", Enabled: true}
	repo := &stubModelPricingRepo{getByIDResult: target}
	svc := newTestModelPricingService(repo)

	svc.mu.Lock()
	svc.cache["gpt-5.1"] = target
	svc.mu.Unlock()

	err := svc.Delete(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, svc.GetCachedPricing("gpt-5.1"), "删除后缓存应清除")
}

func TestDelete_NotFoundReturns404(t *testing.T) {
	repo := &stubModelPricingRepo{getByIDErr: ErrModelPricingNotFound}
	svc := newTestModelPricingService(repo)

	err := svc.Delete(context.Background(), 999)
	require.Error(t, err)
	require.Equal(t, http.StatusNotFound, infraerrors.Code(err))
}
