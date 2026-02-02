# Affiliate 分销系统 - 技术设计文档

**版本**：v1.1
**作者**：Winston (Architect)
**日期**：2026-02-03
**状态**：Draft
**更新记录**：v1.1 修复 binding_type 位置、增加审计日志、完善并发控制

---

## 1. 系统架构

### 1.1 架构概览

```
┌─────────────────────────────────────────────────────────────────────┐
│                           客户端层                                   │
├──────────────────┬──────────────────┬───────────────────────────────┤
│   Web/H5 前端     │    App 客户端     │         管理后台              │
└────────┬─────────┴────────┬─────────┴─────────────┬─────────────────┘
         │                  │                       │
         ▼                  ▼                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         API Gateway                                  │
│                    (认证/限流/路由)                                   │
└─────────────────────────────┬───────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  Affiliate 服务  │←─│   User 服务     │  │  Payment 服务   │
│                 │  │                 │  │        │        │
│ - 邀请码管理     │  │ - 注册时调用    │  │        ↓        │
│ - 关系绑定       │  │   Affiliate    │  │  充值成功后      │
│ - 佣金计算       │  │                 │  │  同步调用        │
│ - 提现处理       │  │                 │  │  Affiliate      │
└────────┬────────┘  └─────────────────┘  └────────┬────────┘
         │                                         │
         └─────────────────┬───────────────────────┘
                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          数据层                                      │
├─────────────────────────────────────────────────────────────────────┤
│          PostgreSQL              │           Redis                  │
│          (主数据存储)             │          (缓存/计数)              │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 模块职责

| 模块 | 职责 |
|------|------|
| **Affiliate 服务** | 分销核心逻辑，邀请关系、佣金计算、提现 |
| **User 服务** | 用户注册时调用 Affiliate 绑定关系 |
| **Payment 服务** | 充值成功后同步调用 Affiliate 计算佣金 |
| **Config 服务** | 分销配置管理，支持热更新 |

### 1.3 设计决策

- **不引入 MQ**：采用同步调用方式，简化架构
- **佣金计算**：在充值流程中同步完成，失败记录重试表
- **绑定类型归属**：`binding_type` 是邀请人的特权，存储在邀请关系表中

---

## 2. 数据模型

### 2.1 ER 图

```
┌──────────────────┐       ┌──────────────────┐
│      users       │       │  affiliate_config │
│──────────────────│       │──────────────────│
│ id               │       │ id               │
│ ...              │       │ config_key       │
└────────┬─────────┘       │ config_value     │
         │                 │ tier_type        │
         │                 └──────────────────┘
         │
         │ 1:1
         ▼
┌──────────────────┐
│ user_affiliate   │
│──────────────────│
│ user_id (PK,FK)  │
│ referral_code    │
│ tier_level       │
│ effective_count  │
│ is_kol           │
│ kol_config       │
│ total_earnings   │
│ withdrawable     │
│ version          │◄── 乐观锁版本号
│ created_at       │
└────────┬─────────┘
         │
         │ 1:N (作为邀请人)
         ▼
┌──────────────────┐
│ referral_relation│◄── 新增：独立邀请关系表
│──────────────────│
│ id               │
│ inviter_id (FK)  │
│ invitee_id (FK)  │
│ binding_type     │◄── 绑定类型移到这里
│ invitee_status   │◄── 被邀请人状态
│ referral_code    │
│ created_at       │
└──────────────────┘
         │
         │ 1:N
         ▼
┌──────────────────┐       ┌──────────────────┐       ┌──────────────────┐
│ commission_record│       │ withdrawal_record│       │affiliate_audit_log│◄── 新增
│──────────────────│       │──────────────────│       │──────────────────│
│ id               │       │ id               │       │ id               │
│ user_id          │       │ user_id          │       │ user_id          │
│ source_user_id   │       │ amount           │       │ action           │
│ source_type      │       │ status           │       │ before_value     │
│ source_order_id  │       │ payment_method   │       │ after_value      │
│ amount           │       │ payment_account  │       │ operator_id      │
│ rate             │       │ reviewed_by      │       │ created_at       │
│ status           │       │ reviewed_at      │       └──────────────────┘
│ confirmed_at     │       │ completed_at     │
│ created_at       │       │ created_at       │
└──────────────────┘       └──────────────────┘
```

### 2.2 表结构详细设计

#### 2.2.1 用户分销信息表 `user_affiliate`

```sql
CREATE TABLE user_affiliate (
    user_id         BIGINT PRIMARY KEY,          -- 关联 users.id
    referral_code   VARCHAR(16) UNIQUE NOT NULL, -- 邀请码
    tier_level      SMALLINT DEFAULT 1,          -- 当前阶梯档位 1/2/3
    effective_count INT DEFAULT 0,               -- 有效邀请数
    is_kol          BOOLEAN DEFAULT FALSE,       -- 是否 KOL
    kol_config      JSONB,                       -- KOL 专属配置
    total_earnings  DECIMAL(12,2) DEFAULT 0,     -- 累计收益
    withdrawable    DECIMAL(12,2) DEFAULT 0,     -- 可提现金额
    version         INT DEFAULT 0,               -- 乐观锁版本号
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_affiliate_code ON user_affiliate(referral_code);
```

**kol_config JSON 结构**：
```json
{
  "promo_code": "KOLDAXIN",
  "commission_rate": 0.10,
  "user_bonus": 2.00,
  "coupon_template_id": 123,
  "default_binding_type": "lifetime"
}
```

> **设计说明**：移除了 `inviter_id` 和 `binding_type`，这两个字段移到独立的 `referral_relation` 表中。KOL 的默认绑定类型存储在 `kol_config.default_binding_type` 中。

#### 2.2.2 邀请关系表 `referral_relation`（新增）

```sql
CREATE TABLE referral_relation (
    id              BIGSERIAL PRIMARY KEY,
    inviter_id      BIGINT NOT NULL,             -- 邀请人 ID
    invitee_id      BIGINT NOT NULL,             -- 被邀请人 ID
    referral_code   VARCHAR(16) NOT NULL,        -- 使用的邀请码
    binding_type    VARCHAR(20) DEFAULT 'first_charge', -- first_charge | lifetime
    invitee_status  VARCHAR(20) DEFAULT 'registered',   -- registered | first_charged | qualified
    first_charge_at TIMESTAMP,                   -- 首充时间
    created_at      TIMESTAMP DEFAULT NOW(),

    FOREIGN KEY (inviter_id) REFERENCES users(id),
    FOREIGN KEY (invitee_id) REFERENCES users(id),
    UNIQUE (invitee_id)                          -- 每个用户只能被邀请一次
);

CREATE INDEX idx_relation_inviter ON referral_relation(inviter_id);
CREATE INDEX idx_relation_invitee ON referral_relation(invitee_id);
CREATE INDEX idx_relation_status ON referral_relation(inviter_id, invitee_status);
```

**字段说明**：

| 字段 | 说明 |
|------|------|
| `binding_type` | 绑定类型，由**邀请人身份**决定（KOL 可设为 lifetime） |
| `invitee_status` | 被邀请人状态，用于判断是否"有效" |
| `first_charge_at` | 首充时间，用于标记转为有效的时间点 |

#### 2.2.3 佣金记录表 `commission_record`

```sql
CREATE TABLE commission_record (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,             -- 获得佣金的用户（邀请人）
    source_user_id  BIGINT NOT NULL,             -- 贡献佣金的用户（被邀请人）
    relation_id     BIGINT,                      -- 关联邀请关系
    source_type     VARCHAR(20) NOT NULL,        -- register | recharge
    source_order_id VARCHAR(64),                 -- 关联订单号（充值场景）
    amount          DECIMAL(10,2) NOT NULL,      -- 佣金金额
    rate            DECIMAL(5,4),                -- 佣金比例
    status          VARCHAR(20) DEFAULT 'pending', -- pending | confirmed | withdrawn | cancelled
    confirmed_at    TIMESTAMP,                   -- 确认时间
    created_at      TIMESTAMP DEFAULT NOW(),

    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (source_user_id) REFERENCES users(id),
    FOREIGN KEY (relation_id) REFERENCES referral_relation(id)
);

CREATE INDEX idx_commission_user ON commission_record(user_id, status);
CREATE INDEX idx_commission_source ON commission_record(source_user_id);
CREATE INDEX idx_commission_created ON commission_record(created_at);
CREATE UNIQUE INDEX idx_commission_order ON commission_record(source_order_id) WHERE source_order_id IS NOT NULL;
```

#### 2.2.4 提现记录表 `withdrawal_record`

```sql
CREATE TABLE withdrawal_record (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,
    amount          DECIMAL(10,2) NOT NULL,
    status          VARCHAR(20) DEFAULT 'pending', -- pending | approved | rejected | completed
    payment_method  VARCHAR(20),                 -- alipay | bank | usdt
    payment_account VARCHAR(128),                -- 加密存储
    reject_reason   VARCHAR(256),
    reviewed_by     BIGINT,                      -- 审核人
    reviewed_at     TIMESTAMP,
    completed_at    TIMESTAMP,
    created_at      TIMESTAMP DEFAULT NOW(),

    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_withdrawal_user ON withdrawal_record(user_id);
CREATE INDEX idx_withdrawal_status ON withdrawal_record(status);
```

#### 2.2.5 分销配置表 `affiliate_config`

```sql
CREATE TABLE affiliate_config (
    id              SERIAL PRIMARY KEY,
    config_key      VARCHAR(64) UNIQUE NOT NULL,
    config_value    JSONB NOT NULL,
    tier_type       VARCHAR(20) DEFAULT 'global', -- global | normal | kol
    description     VARCHAR(256),
    updated_by      BIGINT,
    updated_at      TIMESTAMP DEFAULT NOW()
);
```

**配置项示例**：

| config_key | config_value | 说明 |
|------------|--------------|------|
| `register_bonus` | `{"inviter": 1.00, "invitee": 1.00}` | 注册奖励 |
| `commission_threshold` | `{"type": "or", "monthly_plan": true, "min_spend": 10.00}` | 分成门槛 |
| `tier_rules` | `[{"level": 1, "min": 0, "max": 10, "rate": 0.05}, {"level": 2, "min": 11, "max": 30, "rate": 0.08}, {"level": 3, "min": 31, "max": null, "rate": 0.12}]` | 阶梯规则 |
| `max_commission_rate` | `0.15` | 佣金比例上限 |
| `withdraw_threshold` | `100.00` | 提现门槛 |
| `commission_confirm_days` | `7` | 佣金确认天数 |
| `effective_definition` | `"first_charged"` | 有效用户定义 |

#### 2.2.6 佣金计算失败重试表 `commission_retry`

```sql
CREATE TABLE commission_retry (
    id              BIGSERIAL PRIMARY KEY,
    order_id        VARCHAR(64) NOT NULL,
    user_id         BIGINT NOT NULL,
    amount          DECIMAL(10,2) NOT NULL,
    retry_count     INT DEFAULT 0,
    last_error      TEXT,
    status          VARCHAR(20) DEFAULT 'pending', -- pending | success | failed
    created_at      TIMESTAMP DEFAULT NOW(),
    updated_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_retry_status ON commission_retry(status, retry_count);
```

#### 2.2.7 审计日志表 `affiliate_audit_log`（新增）

```sql
CREATE TABLE affiliate_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL,             -- 被操作的用户
    action          VARCHAR(32) NOT NULL,        -- 操作类型
    entity_type     VARCHAR(32),                 -- 实体类型
    entity_id       BIGINT,                      -- 实体 ID
    before_value    JSONB,                       -- 变更前的值
    after_value     JSONB,                       -- 变更后的值
    operator_id     BIGINT DEFAULT 0,            -- 操作人（0=系统）
    operator_ip     VARCHAR(45),                 -- 操作 IP
    remark          VARCHAR(256),                -- 备注
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_audit_user ON affiliate_audit_log(user_id);
CREATE INDEX idx_audit_action ON affiliate_audit_log(action);
CREATE INDEX idx_audit_created ON affiliate_audit_log(created_at);
```

**action 枚举值**：

| action | 说明 |
|--------|------|
| `relation_created` | 邀请关系建立 |
| `invitee_effective` | 被邀请人变为有效 |
| `commission_created` | 佣金记录创建 |
| `commission_confirmed` | 佣金确认 |
| `commission_cancelled` | 佣金取消 |
| `tier_upgraded` | 阶梯升级 |
| `withdraw_requested` | 申请提现 |
| `withdraw_approved` | 提现审核通过 |
| `withdraw_rejected` | 提现审核拒绝 |
| `withdraw_completed` | 提现完成 |
| `kol_granted` | 授予 KOL 权限 |
| `kol_revoked` | 撤销 KOL 权限 |
| `binding_type_changed` | 绑定类型变更 |

#### 2.2.8 日统计汇总表 `affiliate_daily_stats`（新增，可选）

```sql
CREATE TABLE affiliate_daily_stats (
    id              BIGSERIAL PRIMARY KEY,
    stat_date       DATE NOT NULL,               -- 统计日期
    user_id         BIGINT,                      -- 用户 ID（NULL 表示全局）
    new_invites     INT DEFAULT 0,               -- 新增邀请数
    effective_invites INT DEFAULT 0,             -- 新增有效数
    commission_amount DECIMAL(12,2) DEFAULT 0,   -- 佣金金额
    withdraw_amount DECIMAL(12,2) DEFAULT 0,     -- 提现金额
    created_at      TIMESTAMP DEFAULT NOW(),

    UNIQUE (stat_date, user_id)
);

CREATE INDEX idx_stats_date ON affiliate_daily_stats(stat_date);
CREATE INDEX idx_stats_user ON affiliate_daily_stats(user_id, stat_date);
```

---

## 3. API 设计

### 3.1 用户端 API

#### 3.1.1 获取推广信息

```
GET /api/v1/affiliate/info
```

**Response**:
```json
{
  "code": 0,
  "data": {
    "referral_code": "ABC123",
    "referral_link": "https://example.com/r/ABC123",
    "qrcode_url": "https://cdn.example.com/qr/ABC123.png",
    "tier_level": 2,
    "tier_name": "白银",
    "commission_rate": 0.08,
    "effective_count": 18,
    "next_tier_threshold": 30,
    "total_earnings": 45.00,
    "withdrawable": 32.00,
    "withdraw_threshold": 100.00,
    "can_withdraw": false
  }
}
```

#### 3.1.2 获取邀请记录

```
GET /api/v1/affiliate/invites?page=1&size=20&status=all
```

**Response**:
```json
{
  "code": 0,
  "data": {
    "total": 23,
    "list": [
      {
        "user_id": 10086,
        "nickname": "用户***86",
        "avatar": "https://...",
        "registered_at": "2026-01-15T10:00:00Z",
        "status": "first_charged",
        "first_charge_at": "2026-01-16T08:00:00Z",
        "contributed": 3.50
      }
    ]
  }
}
```

#### 3.1.3 获取佣金记录

```
GET /api/v1/affiliate/commissions?page=1&size=20&status=all
```

**Response**:
```json
{
  "code": 0,
  "data": {
    "total": 45,
    "list": [
      {
        "id": 1001,
        "source_type": "recharge",
        "source_user": "用户***86",
        "amount": 2.50,
        "rate": 0.05,
        "status": "confirmed",
        "created_at": "2026-01-20T15:30:00Z"
      }
    ]
  }
}
```

#### 3.1.4 申请提现

```
POST /api/v1/affiliate/withdraw
```

**Request**:
```json
{
  "amount": 100.00,
  "payment_method": "alipay",
  "payment_account": "138****8888"
}
```

**Response**:
```json
{
  "code": 0,
  "data": {
    "withdraw_id": 2001,
    "status": "pending",
    "estimated_complete": "2026-02-06"
  }
}
```

#### 3.1.5 绑定邀请码（注册时调用）

```
POST /api/v1/affiliate/bind
```

**Request**:
```json
{
  "referral_code": "ABC123"
}
```

### 3.2 管理后台 API

#### 3.2.1 配置管理

```
GET    /api/admin/affiliate/config
PUT    /api/admin/affiliate/config/:key
```

#### 3.2.2 用户管理

```
GET    /api/admin/affiliate/users?keyword=&is_kol=&page=1
GET    /api/admin/affiliate/users/:id
PUT    /api/admin/affiliate/users/:id/kol      -- 设置/取消 KOL
PUT    /api/admin/affiliate/users/:id/binding  -- 设置绑定类型（针对特定关系）
```

#### 3.2.3 提现管理

```
GET    /api/admin/affiliate/withdrawals?status=pending&page=1
PUT    /api/admin/affiliate/withdrawals/:id/approve
PUT    /api/admin/affiliate/withdrawals/:id/reject
```

#### 3.2.4 数据报表

```
GET    /api/admin/affiliate/stats/overview
GET    /api/admin/affiliate/stats/daily?start=&end=
GET    /api/admin/affiliate/stats/kol-ranking
```

#### 3.2.5 审计日志

```
GET    /api/admin/affiliate/audit-logs?user_id=&action=&page=1
```

---

## 4. 核心流程

### 4.1 邀请注册流程

```
┌─────────┐     ┌─────────┐     ┌─────────────┐     ┌─────────────┐
│  用户 B  │     │ Web/App │     │ User Service│     │  Affiliate  │
└────┬────┘     └────┬────┘     └──────┬──────┘     └──────┬──────┘
     │               │                 │                   │
     │ 点击邀请链接   │                 │                   │
     │──────────────>│                 │                   │
     │               │                 │                   │
     │               │ 解析 code       │                   │
     │               │ 存入 cookie     │                   │
     │               │                 │                   │
     │ 填写注册信息   │                 │                   │
     │──────────────>│                 │                   │
     │               │                 │                   │
     │               │ 注册请求+code   │                   │
     │               │────────────────>│                   │
     │               │                 │                   │
     │               │                 │ 创建邀请关系       │
     │               │                 │──────────────────>│
     │               │                 │                   │
     │               │                 │   1. 查询邀请人    │
     │               │                 │   2. 确定binding_type
     │               │                 │   3. 创建relation  │
     │               │                 │   4. 发放注册奖励  │
     │               │                 │   5. 记录审计日志  │
     │               │                 │<──────────────────│
     │               │                 │                   │
     │               │  注册成功+奖励   │                   │
     │<──────────────│<────────────────│                   │
     │               │                 │                   │
```

**绑定类型确定逻辑**：

```go
func DetermineBindingType(inviter *UserAffiliate) string {
    // 1. KOL 使用专属配置
    if inviter.IsKol && inviter.KolConfig != nil {
        if bt := inviter.KolConfig.DefaultBindingType; bt != "" {
            return bt // "lifetime" 或 "first_charge"
        }
    }
    // 2. 默认首充分成
    return "first_charge"
}
```

### 4.2 充值佣金流程（同步版）

```
┌─────────┐     ┌─────────────┐     ┌─────────────┐
│  用户 B  │     │Payment Svc  │     │  Affiliate  │
└────┬────┘     └──────┬──────┘     └──────┬──────┘
     │                 │                   │
     │ 充值成功        │                   │
     │────────────────>│                   │
     │                 │                   │
     │                 │ 同步调用           │
     │                 │ CalculateCommission│
     │                 │──────────────────>│
     │                 │                   │
     │                 │                   │ 1. 查询邀请关系
     │                 │                   │ 2. 检查 binding_type
     │                 │                   │ 3. 检查门槛条件
     │                 │                   │ 4. 计算佣金比例
     │                 │                   │ 5. 创建佣金记录
     │                 │                   │ 6. 更新 invitee_status
     │                 │                   │ 7. 记录审计日志
     │                 │                   │
     │                 │   返回结果        │
     │                 │<──────────────────│
     │                 │                   │
     │ 充值成功响应     │                   │
     │<────────────────│                   │
```

**佣金计算伪代码（更新版）**：

```go
// Payment 充值成功后
func OnRechargeSuccess(order Order) error {
    // 1. 更新订单状态
    updateOrderStatus(order)

    // 2. 同步计算佣金（失败不影响充值结果）
    go func() {
        if err := affiliateService.CalculateCommission(order); err != nil {
            log.Error("佣金计算失败", err)
            // 记录失败，定时任务重试
            saveRetryRecord(order, err)
        }
    }()

    return nil
}

// Affiliate 服务
func (s *AffiliateService) CalculateCommission(order Order) error {
    // 1. 查询邀请关系
    relation, err := s.repo.GetRelationByInvitee(order.UserId)
    if err != nil || relation == nil {
        return nil // 无邀请关系，跳过
    }

    // 2. 检查绑定类型和分成条件
    if relation.BindingType == "first_charge" {
        // 首充模式：检查是否已有充值佣金
        if s.hasRechargeCommission(relation.InviterId, order.UserId) {
            return nil // 已有充值佣金，跳过
        }
    }
    // lifetime 模式：每次充值都计算

    // 3. 获取邀请人信息，检查门槛
    inviter, _ := s.repo.GetAffiliate(relation.InviterId)
    if !s.checkThreshold(inviter) {
        return nil
    }

    // 4. 计算佣金比例
    rate := s.getCommissionRate(inviter)
    amount := order.Amount * rate

    // 5. 开启事务
    return s.repo.Transaction(func(tx *Tx) error {
        // 5.1 创建佣金记录
        commission := CommissionRecord{
            UserId:        relation.InviterId,
            SourceUserId:  order.UserId,
            RelationId:    relation.Id,
            SourceType:    "recharge",
            SourceOrderId: order.OrderId,
            Amount:        amount,
            Rate:          rate,
            Status:        "pending",
        }
        if err := tx.CreateCommission(commission); err != nil {
            return err
        }

        // 5.2 更新被邀请人状态（如果是首充）
        if relation.InviteeStatus == "registered" {
            relation.InviteeStatus = "first_charged"
            relation.FirstChargeAt = time.Now()
            if err := tx.UpdateRelation(relation); err != nil {
                return err
            }

            // 5.3 更新邀请人的有效邀请数（原子操作）
            if err := tx.IncrementEffectiveCount(relation.InviterId); err != nil {
                return err
            }

            // 5.4 检查并更新阶梯
            s.updateTierLevel(tx, relation.InviterId)
        }

        // 5.5 记录审计日志
        tx.CreateAuditLog(AuditLog{
            UserId:     relation.InviterId,
            Action:     "commission_created",
            EntityType: "commission",
            EntityId:   commission.Id,
            AfterValue: toJSON(commission),
        })

        return nil
    })
}
```

### 4.3 有效邀请数更新（原子操作）

```sql
-- 使用 UPDATE ... RETURNING 保证原子性
UPDATE user_affiliate
SET effective_count = effective_count + 1,
    updated_at = NOW()
WHERE user_id = $1
RETURNING effective_count;
```

### 4.4 阶梯升级流程

```go
func (s *AffiliateService) updateTierLevel(tx *Tx, userId int64) error {
    affiliate, _ := tx.GetAffiliate(userId)
    tierRules := s.config.GetTierRules()

    oldTier := affiliate.TierLevel
    newTier := oldTier

    for _, rule := range tierRules {
        if affiliate.EffectiveCount >= rule.Min &&
           (rule.Max == nil || affiliate.EffectiveCount <= *rule.Max) {
            newTier = rule.Level
            break
        }
    }

    if newTier != oldTier {
        affiliate.TierLevel = newTier
        if err := tx.UpdateAffiliate(affiliate); err != nil {
            return err
        }

        // 记录升级日志
        tx.CreateAuditLog(AuditLog{
            UserId:      userId,
            Action:      "tier_upgraded",
            BeforeValue: toJSON(map[string]int{"tier": oldTier}),
            AfterValue:  toJSON(map[string]int{"tier": newTier}),
        })
    }

    return nil
}
```

### 4.5 提现流程（并发控制）

```go
func (s *AffiliateService) RequestWithdraw(userId int64, amount float64) error {
    return s.repo.Transaction(func(tx *Tx) error {
        // 1. 加行锁读取用户分销信息
        affiliate, err := tx.GetAffiliateForUpdate(userId) // SELECT ... FOR UPDATE
        if err != nil {
            return err
        }

        // 2. 检查可提现金额
        if affiliate.Withdrawable < amount {
            return ErrInsufficientBalance
        }

        // 3. 检查提现门槛
        threshold := s.config.GetWithdrawThreshold()
        if amount < threshold {
            return ErrBelowThreshold
        }

        // 4. 使用乐观锁更新余额
        oldVersion := affiliate.Version
        affiliate.Withdrawable -= amount
        affiliate.Version++

        rowsAffected, err := tx.UpdateAffiliateWithVersion(affiliate, oldVersion)
        if err != nil {
            return err
        }
        if rowsAffected == 0 {
            return ErrConcurrentUpdate // 并发冲突，重试
        }

        // 5. 创建提现记录
        withdrawal := WithdrawalRecord{
            UserId: userId,
            Amount: amount,
            Status: "pending",
        }
        if err := tx.CreateWithdrawal(withdrawal); err != nil {
            return err
        }

        // 6. 记录审计日志
        tx.CreateAuditLog(AuditLog{
            UserId:      userId,
            Action:      "withdraw_requested",
            EntityType:  "withdrawal",
            EntityId:    withdrawal.Id,
            BeforeValue: toJSON(map[string]float64{"withdrawable": affiliate.Withdrawable + amount}),
            AfterValue:  toJSON(map[string]float64{"withdrawable": affiliate.Withdrawable}),
        })

        return nil
    })
}
```

**行锁 SQL**：

```sql
-- SELECT FOR UPDATE 加行锁
SELECT * FROM user_affiliate WHERE user_id = $1 FOR UPDATE;

-- 乐观锁更新
UPDATE user_affiliate
SET withdrawable = $1, version = version + 1, updated_at = NOW()
WHERE user_id = $2 AND version = $3;
```

### 4.6 佣金确认流程（定时任务）

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Scheduler  │     │  Affiliate  │     │   Payment   │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ 每日凌晨触发       │                   │
       │──────────────────>│                   │
       │                   │                   │
       │                   │ 查询 pending 且    │
       │                   │ created_at < 7天前 │
       │                   │                   │
       │                   │ 检查订单是否退款   │
       │                   │──────────────────>│
       │                   │                   │
       │                   │<──────────────────│
       │                   │                   │
       │                   │ 未退款：确认佣金   │
       │                   │ 已退款：取消佣金   │
       │                   │                   │
       │                   │ 更新 withdrawable │
       │                   │ 记录审计日志       │
       │                   │                   │
```

### 4.7 失败重试流程（定时任务）

```go
// 每 10 分钟执行
func RetryFailedCommissions() {
    records := repo.GetPendingRetries(maxRetry: 3)

    for _, r := range records {
        order := paymentService.GetOrder(r.OrderId)
        err := affiliateService.CalculateCommission(order)

        if err != nil {
            r.RetryCount++
            r.LastError = err.Error()
            if r.RetryCount >= 3 {
                r.Status = "failed" // 人工处理
                // 发送告警通知
            }
        } else {
            r.Status = "success"
        }

        repo.UpdateRetry(r)
    }
}
```

---

## 5. 缓存策略

### 5.1 缓存设计

| Key 格式 | 数据 | TTL | 更新时机 |
|----------|------|-----|----------|
| `aff:config:{key}` | 配置项 | 5 min | 配置更新时删除 |
| `aff:user:{id}` | 用户分销信息 | 10 min | 数据变更时删除 |
| `aff:code:{code}` | code → user_id 映射 | 24 h | 不变 |
| `aff:relation:{invitee_id}` | 邀请关系 | 1 h | 数据变更时删除 |
| `aff:stats:{id}:daily` | 用户每日统计 | 1 h | 定时刷新 |

### 5.2 热点数据处理

```
KOL 推广码查询（高频）:
1. 先查 Redis aff:code:{code}
2. Miss 则查 DB 并回填
3. KOL 码设置更长 TTL (7d)
```

### 5.3 缓存失效策略

```go
// 数据变更时主动删除缓存
func (s *AffiliateService) InvalidateCache(userId int64) {
    s.cache.Del(fmt.Sprintf("aff:user:%d", userId))
}

func (s *AffiliateService) InvalidateRelationCache(inviteeId int64) {
    s.cache.Del(fmt.Sprintf("aff:relation:%d", inviteeId))
}
```

---

## 6. 安全与风控

### 6.1 防刷策略

| 风险 | 策略 |
|------|------|
| 批量注册 | 设备指纹 + IP 限频（单 IP 24h 最多 5 个注册） |
| 自邀自充 | 禁止邀请人 == 被邀请人，检测关联账号 |
| 虚假充值 | 佣金 7 天确认期，退款则取消 |
| 恶意提现 | 人工审核 + KYC |
| 并发提现 | 行锁 + 乐观锁双重保护 |

### 6.2 数据安全

- 提现账号信息**加密存储**（AES-256）
- 敏感操作记录**审计日志**（全量记录）
- 佣金变动需**幂等处理**（订单号去重）
- 关键操作记录**操作人 IP**

### 6.3 幂等设计

```sql
-- 佣金记录：订单号唯一索引
CREATE UNIQUE INDEX idx_commission_order
ON commission_record(source_order_id)
WHERE source_order_id IS NOT NULL;

-- 邀请关系：被邀请人唯一
UNIQUE (invitee_id)
```

---

## 7. 监控告警

### 7.1 业务指标

| 指标 | 告警阈值 |
|------|----------|
| 单日注册奖励支出 | > $500 |
| 单用户单日佣金 | > $50 |
| 提现失败率 | > 5% |
| 佣金计算延迟 | > 5 min |
| 重试队列堆积 | > 100 条 |

### 7.2 技术指标

- API 响应时间 P99 < 200ms
- 佣金计算任务执行时间 < 30s
- 数据库连接池使用率 < 80%
- 缓存命中率 > 90%

### 7.3 审计日志监控

- 单用户短时间内多次 `tier_upgraded` → 可能是刷量
- 大额 `withdraw_requested` → 自动标记人工审核
- `binding_type_changed` → 通知运营确认

---

## 8. 分期实施建议

### Phase 1（MVP）

- [ ] 数据表创建（含新增表）
- [ ] 邀请码生成与绑定
- [ ] 邀请关系建立
- [ ] 注册奖励发放
- [ ] 首充佣金计算
- [ ] 推广中心基础页面
- [ ] 审计日志记录

### Phase 2

- [ ] 阶梯佣金计算
- [ ] 佣金确认定时任务
- [ ] 提现申请与审核（含并发控制）
- [ ] 后台配置管理
- [ ] 失败重试机制

### Phase 3

- [ ] KOL 体系（专属码、终身绑定）
- [ ] 优惠券关联
- [ ] 数据报表（含日统计汇总）
- [ ] 风控升级

---

## 9. 变更记录

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| v1.0 | 2026-02-03 | 初稿 |
| v1.1 | 2026-02-03 | 1. 新增 `referral_relation` 表，`binding_type` 移至此表<br>2. 新增 `affiliate_audit_log` 审计日志表<br>3. 新增 `affiliate_daily_stats` 日统计表<br>4. `user_affiliate` 增加 `version` 字段支持乐观锁<br>5. 完善提现并发控制逻辑<br>6. 更新佣金计算流程，增加原子操作说明 |
