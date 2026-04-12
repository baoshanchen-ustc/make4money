# Batch 2 — Domain 常量 + Service 领域类型

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** 添加 `RoleDistributor` 常量；在 service 层定义分销相关的领域类型（struct 和 interface），供后续 service/repo/handler 使用。

**Prerequisites:** Batch 1 完成（ent 生成代码存在）

---

### Task 1: domain 常量

**Files:**
- Modify: `backend/internal/domain/constants.go`

- [ ] **Step 1: 在 Role constants 块追加 distributor**

当前文件的 Role 块：
```go
const (
    RoleAdmin = "admin"
    RoleUser  = "user"
)
```

修改为：
```go
const (
    RoleAdmin       = "admin"
    RoleUser        = "user"
    RoleDistributor = "distributor"
)
```

---

### Task 2: service 层 — 分销领域类型

**Files:**
- Create: `backend/internal/service/referral_types.go`

- [ ] **Step 1: 写领域类型文件**

```go
// backend/internal/service/referral_types.go
package service

import (
	"context"
	"time"
)

// ─── 邀请码 ──────────────────────────────────────────────

// ReferralCode 用户专属邀请码领域对象。
type ReferralCode struct {
	ID        int64
	UserID    int64
	Code      string
	CreatedAt time.Time
}

// ─── 邀请关系 ─────────────────────────────────────────────

// ReferralRelation 邀请关系（含快照字段）。
type ReferralRelation struct {
	ID                  int64
	InviterID           int64
	InviteeID           int64
	Level               int8
	InviterRoleSnapshot string
	Status              string // active / disabled
	SignupBonusInviter  float64
	SignupBonusInvitee  float64
	CommissionRate      float64
	CreatedAt           time.Time
}

// ─── 注册奖励事件 ─────────────────────────────────────────

// ReferralSignupRewardEvent outbox 模式的注册奖励事件。
type ReferralSignupRewardEvent struct {
	ID            int64
	RelationID    int64
	BeneficiaryID int64
	RewardType    string // inviter_bonus / invitee_bonus
	Amount        float64
	Status        string // pending / settled / failed
	SettledAt     *time.Time
	ErrorMsg      *string
	RetryCount    int
	CreatedAt     time.Time
}

// ─── 消费返佣事件 ─────────────────────────────────────────

// ReferralCommissionEvent outbox 模式的消费返佣事件。
type ReferralCommissionEvent struct {
	ID               int64
	RelationID       int64
	UsageRequestID   string
	BeneficiaryID    int64
	SourceUserID     int64
	SpendAmount      float64
	CommissionRate   float64
	CommissionAmount float64
	Status           string // pending / settled / failed
	SettledAt        *time.Time
	ErrorMsg         *string
	RetryCount       int
	CreatedAt        time.Time
}

// ─── 分销员申请 ───────────────────────────────────────────

// DistributorApplication 分销员申请领域对象。
type DistributorApplication struct {
	ID         int64
	UserID     int64
	Status     string // pending / approved / rejected
	Reason     *string
	AdminNotes *string
	ReviewedBy *int64
	ReviewedAt *time.Time
	CreatedAt  time.Time
}

// ─── 输入 / 输出 DTO ─────────────────────────────────────

// ReferralStats 用户邀请统计摘要。
type ReferralStats struct {
	TotalInvitees      int64
	TotalSignupBonus   float64
	TotalCommission    float64
}

// ReferralUserInfo GET /user/referral 响应。
type ReferralUserInfo struct {
	Code        string
	InviteURL   string
	Stats       ReferralStats
	Application *DistributorApplication // nil 表示未申请或已是分销员
}

// InviteeStats 分销员查看的下级统计（脱敏）。
type InviteeStats struct {
	UserID            int64
	MaskedEmail       string
	RegisteredAt      time.Time
	Status            string
	TotalSpend        float64 // 聚合自 usage_logs.actual_cost
	RequestCount      int64
	LastActiveAt      *time.Time
	ContributedCommission float64 // 聚合自 referral_commission_events
}

// DistributorOverview 分销员控制台概览。
type DistributorOverview struct {
	TotalInvitees      int64
	TotalSignupBonus   float64
	TotalCommission    float64
}

// ─── 常量 ─────────────────────────────────────────────────

const (
	ReferralStatusActive   = "active"
	ReferralStatusDisabled = "disabled"

	ReferralEventStatusPending  = "pending"
	ReferralEventStatusSettled  = "settled"
	ReferralEventStatusFailed   = "failed"

	ReferralRewardTypeInviterBonus = "inviter_bonus"
	ReferralRewardTypeInviteeBonus = "invitee_bonus"

	DistributorApplicationStatusPending  = "pending"
	DistributorApplicationStatusApproved = "approved"
	DistributorApplicationStatusRejected = "rejected"

	ReferralCodePrefix      = "INV-"
	ReferralCodeMaxRetries  = 5
	ReferralWorkerMaxRetry  = 5
)

// ─── Repository 接口 ─────────────────────────────────────

// ReferralRepository 分销系统数据访问接口。
type ReferralRepository interface {
	// referral_codes
	CreateReferralCode(ctx context.Context, userID int64, code string) (*ReferralCode, error)
	GetReferralCodeByCode(ctx context.Context, code string) (*ReferralCode, error)
	GetReferralCodeByUserID(ctx context.Context, userID int64) (*ReferralCode, error)

	// referral_relations
	CreateRelation(ctx context.Context, rel *ReferralRelation) (*ReferralRelation, error)
	GetDirectRelationByInvitee(ctx context.Context, inviteeID int64) (*ReferralRelation, error)
	GetRelationsByInviter(ctx context.Context, inviterID int64, level int8) ([]*ReferralRelation, error)
	DisableRelationsByUser(ctx context.Context, userID int64) error   // 封禁时调用
	EnableRelationsByUser(ctx context.Context, userID int64) error    // 解封时调用
	ListRelationsAdmin(ctx context.Context, page, pageSize int, inviterID, inviteeID *int64, level *int8, status string) ([]*ReferralRelation, int64, error)

	// referral_signup_reward_events
	CreateSignupRewardEvents(ctx context.Context, events []*ReferralSignupRewardEvent) error
	ListPendingSignupRewardEvents(ctx context.Context, limit int) ([]*ReferralSignupRewardEvent, error)
	UpdateSignupRewardEventStatus(ctx context.Context, id int64, status string, errMsg *string) error
	IncrSignupRewardRetry(ctx context.Context, id int64) error

	// referral_commission_events
	CreateCommissionEvent(ctx context.Context, event *ReferralCommissionEvent) error // 唯一冲突时静默返回 nil
	ListPendingCommissionEvents(ctx context.Context, limit int) ([]*ReferralCommissionEvent, error)
	UpdateCommissionEventStatus(ctx context.Context, id int64, status string, errMsg *string) error
	IncrCommissionRetry(ctx context.Context, id int64) error
	ListCommissionEventsAdmin(ctx context.Context, page, pageSize int, status string) ([]*ReferralCommissionEvent, int64, error)

	// distributor_applications
	CreateApplication(ctx context.Context, app *DistributorApplication) (*DistributorApplication, error)
	GetApplicationByID(ctx context.Context, id int64) (*DistributorApplication, error)
	GetLatestApplicationByUser(ctx context.Context, userID int64) (*DistributorApplication, error)
	UpdateApplication(ctx context.Context, id int64, status, adminNotes string, reviewedBy int64) (*DistributorApplication, error)
	ListApplicationsAdmin(ctx context.Context, page, pageSize int, status string) ([]*DistributorApplication, int64, error)

	// distributor stats（实时聚合）
	GetInviteeStatsList(ctx context.Context, inviterID int64, page, pageSize int, emailSearch string) ([]*InviteeStats, int64, error)
	GetDistributorOverview(ctx context.Context, inviterID int64) (*DistributorOverview, error)
	GetAdminReferralStats(ctx context.Context) (*AdminReferralStats, error)
}

// AdminReferralStats 管理端全局统计。
type AdminReferralStats struct {
	TotalRelations       int64
	TotalSignupBonusPaid float64
	TotalCommissionPaid  float64
}
```

- [ ] **Step 2: 验证编译**

```bash
cd backend
go build ./internal/service/...
```

Expected: 无编译错误（referral_types.go 中所有 interface 方法引用 service 内已有类型，不依赖 Batch 3 的实现）。

- [ ] **Step 3: Commit**

```bash
git add backend/internal/domain/constants.go backend/internal/service/referral_types.go
git commit -m "Feature: 新增 RoleDistributor 常量和分销系统领域类型"
```
