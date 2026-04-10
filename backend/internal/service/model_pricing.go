package service

import (
	"context"
	"time"
)

// ModelPricingEntry 管理员自定义模型计费价格（每百万 token，USD）
type ModelPricingEntry struct {
	ID          int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ModelKey    string // fallback pricing key, e.g. "gpt-5.1"
	DisplayName string // 展示名
	// 每百万 token 价格（USD）— 存储层使用 per-million 方便展示
	InputPricePerMillion             float64
	OutputPricePerMillion            float64
	InputPricePerMillionPriority     float64
	OutputPricePerMillionPriority    float64
	CacheReadPricePerMillion         float64
	CacheReadPricePerMillionPriority float64
	CacheCreationPricePerMillion     float64
	Enabled                          bool
	OverrideLitellm                  bool // 数据库价格优先于 LiteLLM
	Note                             string
}

// ToModelPricing 将 per-million 价格转换为 ModelPricing（per-token）供 BillingService 使用
func (e *ModelPricingEntry) ToModelPricing() *ModelPricing {
	const perMillion = 1e-6
	return &ModelPricing{
		InputPricePerToken:             e.InputPricePerMillion * perMillion,
		OutputPricePerToken:            e.OutputPricePerMillion * perMillion,
		InputPricePerTokenPriority:     e.InputPricePerMillionPriority * perMillion,
		OutputPricePerTokenPriority:    e.OutputPricePerMillionPriority * perMillion,
		CacheReadPricePerToken:         e.CacheReadPricePerMillion * perMillion,
		CacheReadPricePerTokenPriority: e.CacheReadPricePerMillionPriority * perMillion,
		CacheCreationPricePerToken:     e.CacheCreationPricePerMillion * perMillion,
	}
}

// ModelPricingRepository is the persistence interface for model pricing entries.
type ModelPricingRepository interface {
	List(ctx context.Context) ([]ModelPricingEntry, error)
	GetByKey(ctx context.Context, modelKey string) (*ModelPricingEntry, error)
	GetByID(ctx context.Context, id int64) (*ModelPricingEntry, error)
	Create(ctx context.Context, entry *ModelPricingEntry) (*ModelPricingEntry, error)
	Update(ctx context.Context, id int64, entry *ModelPricingEntry) (*ModelPricingEntry, error)
	Delete(ctx context.Context, id int64) error
}
