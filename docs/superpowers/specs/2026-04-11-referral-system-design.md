# 分销邀请系统设计文档（v2）

**日期**: 2026-04-11  
**状态**: 待实现  
**版本**: v3（根据 Codex round-2 review 修订）  
**范围**: 2层分销系统，支持注册奖励 + 消费返佣，引入分销员角色，全后台可配置

---

## 1. 背景与目标

### 现状

项目目前有两套机制，但均不满足需求：

- **邀请码（InvitationCode）**：一次性 `redeem_code`，用于"邀请码注册模式"，本质是准入控制，不绑定邀请关系，不发放奖励。
- **注册优惠码（PromoCode）**：管理员批量生成的运营工具，不绑定邀请人，无法追踪邀请关系。

现有角色：`admin`、`user`，无分销员角色。

### 目标

构建一套独立的 2 层分销系统：

1. **普通用户推广**：任何用户均可通过专属邀请码邀请他人注册，获得注册奖励
2. **分销员推广**：具备 `distributor` 角色的用户，拥有独立奖励规则，并可查看直接下级的脱敏使用统计
3. **全后台可配置**：普通用户和分销员的所有奖励参数独立配置，支持独立开关

### v2 修订说明（根据 Codex round-1 review）

| 问题 | 修复方式 |
|------|---------|
| `invitee_id UNIQUE` 与 2层记录冲突 | 拆分为关系表 + 奖励事件表 |
| 封禁的邀请人仍可收佣 | 返佣时同时检查 inviter 当前状态 |
| 异步返佣无幂等保护 | 复用 `UsageBillingCommand.RequestID` 作为幂等键 |
| level-2 语义不清 | 明确 level-2 仅用于关系链可视，不参与返佣结算 |
| 注册奖励失败无重试机制 | 改为 outbox 事件，由后台任务重试 |
| 计费挂载点描述不准确 | 明确挂载在 `applyUsageBilling` 成功后 |
| 角色修改接口未详细说明 | 补充专用接口、允许转换、验证规则 |
| 分销员统计查询复杂度 | 明确聚合方式（实时查询 + 缓存） |

### v3 修订说明（根据 Codex round-2 review）

| 问题 | 修复方式 |
|------|---------|
| 分销员「消费总额」数据来源错误 | 改为聚合 `usage_logs.actual_cost`，「贡献返佣」仍来自 commission_events |
| 订阅消费是否参与返佣未明确 | 明确：仅 `BalanceCost > 0` 触发返佣，订阅消费不参与 |
| 角色修改 cache invalidation 粒度错误 | 改为 `InvalidateAuthCacheByUserID` |
| `/distributor` 路由包含仅 user 可见的申请记录 | 申请入口和进度统一移至用户个人中心 `/user/referral` |
| `referral_relations` 描述与内容不符 | 表描述改为「关系表，存图结构 + 建立时策略快照」 |
| 残留对已删除 `*_commission_rate_l2` 配置的引用 | 删除该段说明，改为简洁表述 |

---

## 2. 角色体系

### 2.1 新增角色：`distributor`（分销员）

| 角色 | 值 | 说明 |
|------|----|------|
| 管理员 | `admin` | 现有，不变 |
| 普通用户 | `user` | 现有，不变 |
| **分销员** | `distributor` | **新增**，介于 user 和 admin 之间 |

分销员**不能访问管理后台**，拥有独立的分销员控制台（`/distributor`）。

### 2.2 分销员身份获取

**方式 A：管理员直接指定**（`PUT /api/v1/admin/users/:id/role`，专用接口）

**方式 B：用户申请 → 管理员审核**

允许的角色转换：
- `user → distributor`（升级）
- `distributor → user`（降级）
- `admin` 不在此接口范围内，不可互转

### 2.3 降级后的语义

分销员降级为普通用户后：
- **历史 `referral_relations` 记录保留**，不变更
- 已有关系的 `inviter_role_snapshot` 快照不变，历史返佣比例不受影响
- **新产生的消费**：现有关系中的 `commission_rate` 已快照，仍按快照值结算
- 新邀请的下级将使用降级后角色（`user`）对应的规则快照

---

## 3. 分销层级模型

```
L1（邀请人，user 或 distributor）
 └── L2（L1 的直接下级）
      └── L3（L2 的直接下级）
```

**layer-2 语义**（重要）：

`level=2` 关系仅用于**关系链可视化和报表**，**不参与任何返佣结算**。

返佣结算规则：
- L2 消费 → L1 获返佣（level=1 关系）
- L3 消费 → L2 获返佣（level=1 关系，从 L3 视角看 L2 是其直接邀请人）
- L1 不参与 L3 的消费返佣

返佣只发生在 level=1 关系（直接邀请人），每个角色只有一套 `commission_rate` 配置，不存在第二套比例。

---

## 4. 数据模型

### 4.1 新增表：`referral_codes`

用户专属邀请码，每用户一条，注册时自动生成。

```sql
CREATE TABLE referral_codes (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL UNIQUE,
    code        VARCHAR(32) NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.2 新增表：`referral_relations`（关系表，存图结构 + 建立时策略快照）

记录邀请关系链，以及建立关系时的奖励策略快照（金额、返佣比例），不存储聚合统计数据。

```sql
CREATE TABLE referral_relations (
    id                    BIGSERIAL PRIMARY KEY,
    inviter_id            BIGINT NOT NULL,
    invitee_id            BIGINT NOT NULL,
    level                 SMALLINT NOT NULL,     -- 1=直接邀请, 2=间接邀请（仅报表用）
    inviter_role_snapshot VARCHAR(20) NOT NULL,  -- 邀请建立时的角色快照
    status                VARCHAR(20) NOT NULL DEFAULT 'active',  -- active / disabled

    -- 注册时快照奖励配置（用于 signup_reward_events 的金额基准）
    signup_bonus_inviter  DECIMAL(20,8) NOT NULL DEFAULT 0,
    signup_bonus_invitee  DECIMAL(20,8) NOT NULL DEFAULT 0,
    commission_rate       DECIMAL(8,4) NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_referral_relations_inviter_invitee_level
        UNIQUE (inviter_id, invitee_id, level)
);

CREATE INDEX idx_referral_relations_inviter ON referral_relations (inviter_id);
CREATE INDEX idx_referral_relations_invitee ON referral_relations (invitee_id);
CREATE INDEX idx_referral_relations_status  ON referral_relations (status);
```

> **设计说明**：
> - `invitee_id` 不再是 UNIQUE，允许同一被邀请人在表中有 level=1 和 level=2 两条记录（分别代表直接和间接关系）
> - 唯一约束改为 `(inviter_id, invitee_id, level)` 防止重复写入
> - 金额快照字段保留在此表，供奖励事件表引用，但聚合统计不在此表维护

### 4.3 新增表：`referral_signup_reward_events`（注册奖励事件表）

每次成功发放的注册奖励记录一条，支持重试和对账。

```sql
CREATE TABLE referral_signup_reward_events (
    id              BIGSERIAL PRIMARY KEY,
    relation_id     BIGINT NOT NULL,        -- 关联 referral_relations.id
    beneficiary_id  BIGINT NOT NULL,        -- 获得奖励的用户 id
    reward_type     VARCHAR(20) NOT NULL,   -- inviter_bonus / invitee_bonus
    amount          DECIMAL(20,8) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending / settled / failed
    settled_at      TIMESTAMPTZ,
    error_msg       TEXT,                   -- 失败原因
    retry_count     INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_signup_reward_relation_type
        UNIQUE (relation_id, reward_type)   -- 每条关系每种奖励类型只发一次
);

CREATE INDEX idx_signup_reward_status    ON referral_signup_reward_events (status);
CREATE INDEX idx_signup_reward_beneficiary ON referral_signup_reward_events (beneficiary_id);
```

### 4.4 新增表：`referral_commission_events`（消费返佣事件表）

每次消费产生的返佣记录，幂等键为 `(usage_request_id, beneficiary_id)`。

```sql
CREATE TABLE referral_commission_events (
    id               BIGSERIAL PRIMARY KEY,
    relation_id      BIGINT NOT NULL,        -- 关联 referral_relations.id（level=1）
    usage_request_id VARCHAR(128) NOT NULL,  -- 复用 UsageBillingCommand.RequestID
    beneficiary_id   BIGINT NOT NULL,        -- 获得返佣的用户 id（inviter）
    source_user_id   BIGINT NOT NULL,        -- 产生消费的用户 id（invitee）
    spend_amount     DECIMAL(20,8) NOT NULL, -- 本次消费金额
    commission_rate  DECIMAL(8,4) NOT NULL,  -- 使用的比例快照
    commission_amount DECIMAL(20,8) NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending / settled / failed
    settled_at       TIMESTAMPTZ,
    error_msg        TEXT,
    retry_count      INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_commission_request_beneficiary
        UNIQUE (usage_request_id, beneficiary_id)  -- 一次计费事件对一个受益人只结算一次
);

CREATE INDEX idx_commission_events_status       ON referral_commission_events (status);
CREATE INDEX idx_commission_events_beneficiary  ON referral_commission_events (beneficiary_id);
CREATE INDEX idx_commission_events_relation     ON referral_commission_events (relation_id);
```

### 4.5 新增表：`distributor_applications`（分销员申请表）

```sql
CREATE TABLE distributor_applications (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending / approved / rejected
    reason       TEXT,
    admin_notes  TEXT,
    reviewed_by  BIGINT,
    reviewed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_distributor_applications_user_id ON distributor_applications (user_id);
CREATE INDEX idx_distributor_applications_status  ON distributor_applications (status);
```

### 4.6 后台配置（扩展现有 `settings` 表）

**普通用户（user）规则**：

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `referral_enabled` | bool | false | 分销功能总开关 |
| `referral_user_signup_bonus_enabled` | bool | false | 普通用户注册奖励开关 |
| `referral_user_signup_bonus_inviter` | float | 0 | 普通用户邀请人注册奖励（元） |
| `referral_user_signup_bonus_invitee` | float | 0 | 普通用户被邀请人注册奖励（元） |
| `referral_user_commission_enabled` | bool | false | 普通用户消费返佣开关 |
| `referral_user_commission_rate` | float | 0 | 普通用户作为直接邀请人的返佣比例 |

**分销员（distributor）规则**：

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `referral_dist_signup_bonus_enabled` | bool | false | 分销员注册奖励开关 |
| `referral_dist_signup_bonus_inviter` | float | 0 | 分销员邀请人注册奖励（元） |
| `referral_dist_signup_bonus_invitee` | float | 0 | 分销员被邀请人注册奖励（元） |
| `referral_dist_commission_enabled` | bool | false | 分销员消费返佣开关 |
| `referral_dist_commission_rate` | float | 0 | 分销员作为直接邀请人的返佣比例 |

> **配置简化说明**：v1 中的 `commission_rate_l1` / `commission_rate_l2` 合并为单一 `commission_rate`，因为返佣只发生在 level=1 关系（直接邀请人），不存在第二套比例。

---

## 5. 核心流程

### 5.1 专属码生成

用户**注册成功时**自动生成：
1. 生成规则：`INV-` + 8位大写字母数字随机串（如 `INV-A3K9XZ8M`）
2. 冲突重试：最多 5 次
3. 在注册事务内写入

### 5.2 注册流程

```
前端注册请求（新增 referral_code 字段）
  ├─ URL ?ref=xxx → 自动填入表单（可手动修改）
  └─ 用户手动填写

后端处理（在注册事务内）：
  1. referral_code 为空 → 仅生成专属码，不建立邀请关系
  2. referral_code 不为空：
     a. 查 referral_codes 找邀请人（L1）
     b. 校验 L1 用户状态为 active（封禁用户不能成为邀请人）
     c. 校验 L1 != 新用户（防止自邀）
     d. 注册成功后（同一事务）：
        i.  生成新用户专属码，写 referral_codes
        ii. 读取 L1 角色 → 按角色读取对应配置，快照奖励金额和返佣比例
        iii. 写 referral_relations（inviter=L1, invitee=新用户, level=1）
        iv. 若 L1 本身有直接邀请人 L0（查 level=1 WHERE invitee_id=L1）：
             写 referral_relations（inviter=L0, invitee=新用户, level=2，
                                    commission_rate=0，不参与返佣结算）
        v.  写 referral_signup_reward_events（status=pending）：
             - 若奖励开关开启：inviter_bonus（L1）+ invitee_bonus（新用户）
             - 若开关关闭：写入 amount=0 的记录（用于对账完整性）

后台 outbox worker 处理 pending 奖励事件：
  - 查 referral_signup_reward_events WHERE status=pending
  - 对每条记录：
      检查 beneficiary 用户状态为 active
      余额 += amount
      status = settled，settled_at = now()
  - 失败时：error_msg 记录原因，retry_count++，status 保持 pending
  - 超过 max_retry（如 5 次）后 status = failed，告警
```

### 5.3 消费返佣流程

挂载点：`applyUsageBilling` 返回 `applied=true` 之后（即计费确认成功后），异步执行。

**返佣仅适用于余额扣费（`BalanceCost > 0`）**。通过订阅套餐消费的用量（`SubscriptionCost`）不产生分销返佣，这是有意设计：订阅用量已通过套餐定价体现，不应再叠加返佣成本。

```
applyUsageBilling 成功（billing applied=true）且 BalanceCost > 0
  │
  └─ 异步触发 referral commission worker：
       参数：usage_request_id（即 UsageBillingCommand.RequestID），
             user_id（消费用户），spend_amount（BalanceCost）
       
       1. 查 referral_relations WHERE invitee_id=user_id AND level=1 AND status=active
       2. 未找到 → 结束
       3. 找到（inviter=L1, commission_rate=快照值）：
          a. 检查 L1 当前用户状态为 active（封禁的邀请人不收佣）
          b. 检查对应角色的 commission_enabled 开关
          c. commission = spend_amount × commission_rate
          d. 若 commission <= 0 → 结束
          e. 尝试写 referral_commission_events：
             - 若 UNIQUE(usage_request_id, beneficiary_id) 冲突 → 幂等，已处理，结束
             - 否则写入 status=pending
       
       后台 outbox worker 处理 pending 返佣事件：
       - 检查 beneficiary 状态仍为 active
       - L1 余额 += commission_amount
       - status = settled
       - 失败时 retry，超过上限 status = failed，告警
```

> **幂等保证**：`(usage_request_id, beneficiary_id)` 唯一约束确保同一计费事件对同一受益人只结算一次，即使 worker 重试或手动重放也安全。

### 5.4 生命周期事件处理

**用户被封禁时**：
1. 将其所有 `referral_relations`（作为 invitee）`status` 置为 `disabled`（停止为上级产生返佣）
2. 将其所有 `referral_relations`（作为 inviter）`status` 置为 `disabled`（停止为下级产生奖励资格）
3. 已 `settled` 的奖励不追回
4. `pending` 的返佣事件：worker 处理时检查 inviter 状态，active 才发放

**用户被解封时**：
1. 将其 `referral_relations` 重新置为 `active`（恢复）

**分销员降级为普通用户时**：
1. `referral_relations` 记录不变（`inviter_role_snapshot` 快照保留）
2. 现有关系的 `commission_rate` 快照不变，持续按快照值结算
3. 新邀请的下级（未来注册）按当时的普通用户规则快照

**配置修改时**：
- 仅影响之后建立的新关系（新注册时快照新配置）
- 已有关系的快照字段不变

---

## 6. 分销员控制台

分销员（`distributor` 角色）独立控制台（`/distributor`），与管理后台完全隔离。

### 6.1 下级统计数据（仅 L2，脱敏）

| 字段 | 说明 |
|------|------|
| 邮箱 | 脱敏（`u***@gmail.com`） |
| 注册时间 | 完整显示 |
| 状态 | active / disabled |
| 消费总额 | 聚合自 `usage_logs.actual_cost`（含订阅和余额消费） |
| 调用次数 | 聚合自 `usage_logs` |
| 最近活跃时间 | `usage_logs` 最新记录时间 |
| 为我贡献返佣 | 聚合自 `referral_commission_events`（仅余额扣费部分） |

**查询实现**：实时聚合 + Redis 缓存（TTL 5 分钟），避免每次全表扫描。

**不可见**：prompt 内容、具体调用记录、API Key、余额、充值记录。

### 6.2 控制台页面结构

```
/distributor                   （仅 distributor 角色可访问）
├── 概览（总邀请人数、累计注册奖励、累计返佣）
├── 我的邀请码（专属码 + 一键复制链接）
└── 下级列表（L2 统计，分页，邮箱模糊搜索）
```

> 分销员申请入口和申请进度查看放在**用户个人中心**（`/user/referral`），不在 `/distributor` 路由下。`/distributor` 仅对已通过审核的 `distributor` 角色开放。

---

## 7. API 设计

### 7.1 用户端 API（所有登录用户）

#### `GET /api/v1/user/referral`
获取当前用户邀请码和基础统计。

```json
{
  "code": "INV-A3K9XZ8M",
  "invite_url": "https://example.com/register?ref=INV-A3K9XZ8M",
  "stats": {
    "total_invitees": 3,
    "total_signup_bonus": 6.00,
    "total_commission": 0.00
  }
}
```

#### `POST /api/v1/user/referral/apply-distributor`
提交分销员申请（仅 `user` 角色，有 `pending` 申请时拒绝重复提交）。

```json
{ "reason": "申请原因..." }
```

### 7.2 分销员 API（`distributor` 角色，middleware 守卫）

#### `GET /api/v1/distributor/overview`
概览统计。

#### `GET /api/v1/distributor/invitees`
直接下级（L2）脱敏统计列表，分页，支持邮箱模糊搜索。

### 7.3 管理端 API

#### `GET/PUT /api/v1/admin/referral/settings`
读取/更新分销配置。

#### `GET /api/v1/admin/referral/relations`
邀请关系列表，支持按邀请人/被邀请人/层级/状态筛选，分页。

#### `GET /api/v1/admin/referral/commission-events`
返佣事件列表，含 settled / pending / failed 状态筛选。

#### `GET /api/v1/admin/referral/stats`
全局统计：总关系数、总注册奖励已发放、总返佣已发放。

#### `GET /api/v1/admin/distributor/applications`
分销员申请列表。

#### `PUT /api/v1/admin/distributor/applications/:id`
审批申请（approve / reject + admin_notes）。

#### `PUT /api/v1/admin/users/:id/role`
专用角色修改接口（**新增，独立于现有用户更新接口**）。

**请求体**：
```json
{ "role": "distributor" }
```

**验证规则**：
- 仅允许 `user ↔ distributor` 互转
- `admin` 角色不可通过此接口修改
- 修改后调用 `InvalidateAuthCacheByUserID` 使该用户所有 API key 的 auth cache 失效

---

## 8. 文件变更清单

### 后端新增
- `backend/ent/schema/referral_code.go`
- `backend/ent/schema/referral_relation.go`
- `backend/ent/schema/referral_signup_reward_event.go`
- `backend/ent/schema/referral_commission_event.go`
- `backend/ent/schema/distributor_application.go`
- `backend/migrations/087_referral_system.sql`
- `backend/internal/service/referral_service.go`
- `backend/internal/repository/referral_repo.go`
- `backend/internal/handler/user/referral_handler.go`
- `backend/internal/handler/distributor/overview_handler.go`
- `backend/internal/handler/admin/referral_handler.go`
- `backend/internal/handler/admin/distributor_application_handler.go`
- `backend/internal/server/middleware/distributor_only.go`
- `backend/internal/service/referral_reward_worker.go`   — outbox worker（注册奖励 + 返佣结算）

### 后端修改
- `backend/internal/domain/constants.go` — 新增 `RoleDistributor = "distributor"`
- `backend/internal/service/domain_constants.go` — 同步常量
- `backend/ent/schema/user.go` — 新增 referral 相关 edges
- `backend/internal/service/auth_service.go` — 注册时生成专属码、写关系、写奖励事件
- `backend/internal/service/gateway_service.go` — `applyUsageBilling` 成功后触发返佣 worker
- `backend/internal/service/setting_service.go` — 新增分销配置项
- `backend/internal/service/admin_service.go` — 封禁时联动 referral_relations；新增角色修改方法
- `backend/internal/setup/handler.go` — 注册新路由

### 前端新增
- `frontend/src/views/user/ReferralView.vue` — 用户邀请页：专属码、邀请统计、申请分销员入口 + 申请进度
- `frontend/src/views/distributor/DistributorLayout.vue`
- `frontend/src/views/distributor/OverviewView.vue`
- `frontend/src/views/distributor/InviteesView.vue`
- `frontend/src/views/admin/ReferralSettingsView.vue`
- `frontend/src/views/admin/ReferralRelationsView.vue`
- `frontend/src/views/admin/DistributorApplicationsView.vue`
- `frontend/src/api/user/referral.ts`
- `frontend/src/api/distributor/index.ts`
- `frontend/src/api/admin/referral.ts`

### 前端修改
- `frontend/src/views/auth/RegisterView.vue` — 新增邀请码输入框，读取 `?ref=` 参数
- `frontend/src/router/index.ts` — 新增路由和 distributor 路由守卫
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

---

## 9. 边界条件与防护

| 场景 | 处理方式 |
|------|---------|
| 用户填写自己的邀请码 | 后端拒绝（inviter == invitee） |
| 邀请码不存在 | 静默忽略，正常注册，无邀请关系 |
| 邀请人注册时已被封禁 | 静默忽略，不建立邀请关系 |
| 注册奖励发放失败 | outbox worker 重试，超限后 failed 告警，不影响注册 |
| 返佣时 inviter 已被封禁 | worker 检查 inviter 状态，跳过发放 |
| 并发注册同一邀请码 | `(inviter_id, invitee_id, level)` 唯一约束保证幂等 |
| 返佣重复结算 | `(usage_request_id, beneficiary_id)` 唯一约束阻断 |
| 重复提交分销员申请 | 有 pending 申请时返回 409 |
| 分销员降级后的返佣 | 历史快照不变，按快照值继续结算 |
| 分销员查看非直属下级 | API 强制过滤 level=1 关系，后端执行 |
| 配置修改后的影响 | 仅影响新建关系，存量快照不变 |

---

## 10. 不在本期范围内

- 提现/结算（返佣仅入余额）
- 3层及以上分销
- 分销员手动管理下级返佣资格
- 返佣独立对账报表
- 防刷机制（IP 限制、手机号验证等）
- 分销员查看 L3 数据
