package service

import (
	"context"
	"strings"
	"sync"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrModelPricingNotFound 是模型价格记录不存在时返回的错误，映射到 HTTP 404。
var ErrModelPricingNotFound = infraerrors.NotFound("MODEL_PRICING_NOT_FOUND", "model pricing not found")

// ErrModelPricingExists 是重复 model_key 冲突时返回的错误，映射到 HTTP 409。
var ErrModelPricingExists = infraerrors.Conflict("MODEL_PRICING_EXISTS", "a pricing entry with this model_key already exists")

// ModelPricingService 管理模型计费价格配置。
// 提供 CRUD 操作，并维护内存缓存供 BillingService 高效查询。
type ModelPricingService struct {
	repo ModelPricingRepository

	mu    sync.RWMutex
	cache map[string]*ModelPricingEntry // model_key -> entry，仅缓存 enabled=true
}

func NewModelPricingService(repo ModelPricingRepository) *ModelPricingService {
	return &ModelPricingService{
		repo:  repo,
		cache: make(map[string]*ModelPricingEntry),
	}
}

// LoadCache 启动时预加载所有 enabled=true 的价格到内存缓存。
func (s *ModelPricingService) LoadCache(ctx context.Context) error {
	entries, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	m := make(map[string]*ModelPricingEntry, len(entries))
	for i := range entries {
		if entries[i].Enabled {
			e := entries[i]
			m[e.ModelKey] = &e
		}
	}
	s.mu.Lock()
	s.cache = m
	s.mu.Unlock()
	return nil
}

// GetCachedPricing 从内存缓存查询某个 modelKey 的计费价格（per-token）。
// 返回 nil 表示未命中缓存。
func (s *ModelPricingService) GetCachedPricing(modelKey string) *ModelPricing {
	s.mu.RLock()
	entry, ok := s.cache[modelKey]
	s.mu.RUnlock()
	if !ok || entry == nil {
		return nil
	}
	return entry.ToModelPricing()
}

// List 返回所有价格条目（含 disabled）。
func (s *ModelPricingService) List(ctx context.Context) ([]ModelPricingEntry, error) {
	return s.repo.List(ctx)
}

// Create 新建价格条目并刷新缓存。
func (s *ModelPricingService) Create(ctx context.Context, entry *ModelPricingEntry) (*ModelPricingEntry, error) {
	normalizeModelKey(entry)
	if err := validatePricingEntry(entry); err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, entry)
	if err != nil {
		return nil, err
	}
	s.updateCacheEntry(created)
	return created, nil
}

// Update 更新价格条目并刷新缓存。
func (s *ModelPricingService) Update(ctx context.Context, id int64, entry *ModelPricingEntry) (*ModelPricingEntry, error) {
	normalizeModelKey(entry)
	if err := validatePricingEntry(entry); err != nil {
		return nil, err
	}

	// 更新前先读出旧记录，用于处理 model_key 被修改时的缓存失效。
	old, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, id, entry)
	if err != nil {
		return nil, err
	}

	// 若 model_key 发生变更，须先清除旧 key 的缓存，防止旧模型继续命中陈旧价格。
	if old != nil && old.ModelKey != updated.ModelKey {
		s.mu.Lock()
		delete(s.cache, old.ModelKey)
		s.mu.Unlock()
	}

	s.updateCacheEntry(updated)
	return updated, nil
}

// Delete 删除价格条目并刷新缓存。
func (s *ModelPricingService) Delete(ctx context.Context, id int64) error {
	// 先查出 modelKey，删除后清除缓存。
	old, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, old.ModelKey)
	s.mu.Unlock()
	return nil
}

func (s *ModelPricingService) updateCacheEntry(entry *ModelPricingEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry.Enabled {
		s.cache[entry.ModelKey] = entry
	} else {
		delete(s.cache, entry.ModelKey)
	}
}

// validatePricingEntry 校验入参合法性：
//   - model_key 规范化后不能为空
//   - 允许全 0 价格（用于免费模型）
func validatePricingEntry(entry *ModelPricingEntry) error {
	if entry.ModelKey == "" {
		return infraerrors.BadRequest("MODEL_PRICING_EMPTY_KEY", "model_key must not be empty")
	}
	return nil
}

// normalizeModelKey 规范化 model_key：去除首尾空白并转小写。
// BillingService 在查缓存前会将模型名转为小写，因此存储非小写 key 会导致配置永远不命中。
func normalizeModelKey(entry *ModelPricingEntry) {
	entry.ModelKey = strings.ToLower(strings.TrimSpace(entry.ModelKey))
}

// GetByKey 按 model_key 精确查询价格条目（含 disabled）。
// 未找到返回 nil, nil。
func (s *ModelPricingService) GetByKey(ctx context.Context, modelKey string) (*ModelPricingEntry, error) {
	return s.repo.GetByKey(ctx, modelKey)
}
