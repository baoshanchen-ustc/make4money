---
stepsCompleted: [1, 2, 3]
inputDocuments:
  - _bmad-output/affiliate/PRD-affiliate-system.md
  - _bmad-output/affiliate/TDD-affiliate-system.md
  - _bmad-output/affiliate/UX-affiliate-system.md
---

# Affiliate 分销系统 - Epic Breakdown

## Overview

本文档为 Affiliate 分销系统提供完整的 Epic 和 Story 拆分，将 PRD、架构设计和 UX 设计中的需求分解为可实施的用户故事。

---

## Requirements Inventory

### Functional Requirements

#### 邀请机制类

**FR1**: 每个用户可生成专属邀请链接，格式：`{domain}/r/{code}`

**FR2**: 邀请码为 6-8 位字母数字组合，系统自动生成

**FR3**: KOL 用户支持自定义专属推广码

**FR4**: 基于邀请链接生成二维码图片，支持下载

**FR5**: 提供预设分享海报模板（3套：简约风、活力风、节日风），含二维码和利益点文案

**FR6**: 新用户通过邀请链接注册时自动绑定邀请关系

**FR7**: 邀请关系绑定后不可更改

**FR8**: 同一设备/IP 24 小时内限制注册账号数量（防刷）

#### 注册奖励类

**FR9**: 被邀请人完成注册时，邀请人获得配置金额的余额奖励

**FR10**: 被邀请人完成注册时，自己获得配置金额的余额奖励

**FR11**: 注册奖励金额可在后台配置（邀请人/被邀请人分别配置）

#### 佣金分成类

**FR12**: 首充分成模式：被邀请人首次充值时，邀请人获得一次性佣金

**FR13**: 终身绑定模式：被邀请人每次充值，邀请人均获得佣金（KOL 专属功能）

**FR14**: 邀请人需满足分成门槛条件才能获得充值佣金

**FR15**: 分成门槛条件：已购买过包月套餐，或历史消费累计满配置金额（两者满足其一）

**FR16**: 阶梯佣金机制：根据有效邀请人数，佣金比例递增（如：0-10人5%，11-30人8%，31+人12%）

**FR17**: 佣金计算公式：佣金 = 被邀请人充值金额 × 邀请人当前佣金比例

**FR18**: 有效邀请定义：被邀请人完成首次充值

#### KOL 体系类

**FR19**: 管理员可将指定用户设置为 KOL

**FR20**: KOL 可拥有专属推广码（支持自定义，如 `KOLDAXIN`）

**FR21**: KOL 可设置专属佣金比例（不受阶梯规则限制）

**FR22**: KOL 可开启终身绑定分成模式

**FR23**: KOL 可关联专属优惠券模板

**FR24**: 通过 KOL 推广码注册的用户可获得额外注册奖励（后台配置）

**FR25**: 通过 KOL 推广码注册的用户可获得首充优惠券

#### 提现系统类

**FR26**: 用户累计已确认佣金达到提现门槛后可申请提现

**FR27**: 提现门槛金额可在后台配置（默认 $100）

**FR28**: 提现方式支持微信、支付宝

**FR29**: 提现申请需人工审核

**FR30**: 审核通过后 T+3 个工作日到账

**FR31**: 佣金状态流转：待确认 → 已确认 → 提现中 → 已提现

**FR32**: 佣金确认周期：被邀请人充值后 7 天无退款则自动确认

**FR33**: 充值订单退款时，对应佣金自动取消

#### 推广中心（用户端）类

**FR34**: 用户可查看累计收益总额

**FR35**: 用户可查看可提现金额

**FR36**: 用户可查看提现进度条（距离提现门槛还差多少）

**FR37**: 用户可查看邀请人数（总数/有效数）

**FR38**: 用户可查看当前阶梯档位和佣金比例

**FR39**: 用户可查看阶梯升级进度条（再邀请多少人升级）

**FR40**: 用户可一键复制邀请链接（复制成功 Toast 提示）

**FR41**: 用户可生成分享海报（3套模板可选）

**FR42**: 用户可下载专属二维码图片

**FR43**: 用户可查看邀请记录列表（支持筛选：全部/有效/待激活）

**FR44**: 用户可查看佣金明细列表（支持筛选：全部/待确认/已确认）

**FR45**: 用户可查看提现记录列表

**FR46**: 用户可查看推广规则说明和 FAQ

#### 后台管理类

**FR47**: 管理员可配置注册奖励金额（邀请人/被邀请人）

**FR48**: 管理员可配置分成门槛条件（包月套餐开关、最低消费额）

**FR49**: 管理员可配置阶梯档位和对应佣金比例

**FR50**: 管理员可配置佣金比例上限

**FR51**: 管理员可配置提现门槛金额

**FR52**: 管理员可配置佣金确认周期（天数）

**FR53**: 管理员可搜索查看用户分销数据

**FR54**: 管理员可设置/取消用户的 KOL 身份

**FR55**: 管理员可为 KOL 配置专属推广码

**FR56**: 管理员可为 KOL 配置专属佣金比例

**FR57**: 管理员可为 KOL 开启/关闭终身绑定

**FR58**: 管理员可为 KOL 关联优惠券模板

**FR59**: 管理员可查看分销概览报表（邀请数、有效数、佣金总额）

**FR60**: 管理员可查看 KOL 排行榜

**FR61**: 管理员可查看提现申请列表（支持筛选状态）

**FR62**: 管理员可审核通过提现申请

**FR63**: 管理员可拒绝提现申请（需填写拒绝原因）

**FR64**: 管理员可查看审计日志

---

### NonFunctional Requirements

#### 安全性

**NFR1**: 设备指纹 + IP 限频防止批量注册羊毛党（单 IP 24h 最多 5 个注册）

**NFR2**: 禁止自邀自充（邀请人不能是被邀请人本人，检测关联账号）

**NFR3**: 佣金 7 天确认期防止虚假充值套利

**NFR4**: 提现需人工审核 + KYC 验证

**NFR5**: 提现账号信息加密存储（AES-256）

**NFR6**: 所有敏感操作记录审计日志（关系建立、佣金变动、提现操作、KOL 设置等）

**NFR7**: 关键操作记录操作人 IP

#### 幂等性与并发控制

**NFR8**: 佣金计算幂等处理（订单号唯一索引去重）

**NFR9**: 邀请关系被邀请人唯一约束（每个用户只能被邀请一次）

**NFR10**: 提现并发控制：行锁（SELECT FOR UPDATE）+ 乐观锁（version 字段）双重保护

**NFR11**: 有效邀请数更新使用原子操作（UPDATE ... RETURNING）

#### 性能

**NFR12**: API 响应时间 P99 < 200ms

**NFR13**: 佣金计算任务执行时间 < 30s

**NFR14**: 数据库连接池使用率 < 80%

**NFR15**: 缓存命中率 > 90%

#### 可靠性

**NFR16**: 佣金计算失败时记录重试表，定时任务重试（最多 3 次）

**NFR17**: 佣金计算失败不影响充值主流程

#### 监控告警

**NFR18**: 单日注册奖励支出超 $500 告警（防刷监控）

**NFR19**: 单用户单日佣金超 $50 告警（异常监控）

**NFR20**: 提现失败率 > 5% 告警

**NFR21**: 佣金计算延迟 > 5 min 告警

**NFR22**: 重试队列堆积 > 100 条告警

---

### Additional Requirements

#### 数据库需求（来自架构文档）

- **user_affiliate 表**：用户分销信息（邀请码、阶梯档位、有效邀请数、KOL 配置、收益、可提现金额、乐观锁版本号）
- **referral_relation 表**：邀请关系（邀请人、被邀请人、绑定类型、被邀请人状态、首充时间）
- **commission_record 表**：佣金记录（用户、来源用户、来源类型、订单号、金额、比例、状态、确认时间）
- **withdrawal_record 表**：提现记录（用户、金额、状态、提现方式、收款账户、审核人、审核时间）
- **affiliate_config 表**：分销配置（配置键、配置值 JSON、配置类型）
- **commission_retry 表**：佣金计算失败重试（订单号、用户、金额、重试次数、错误信息、状态）
- **affiliate_audit_log 表**：审计日志（用户、操作类型、实体类型、变更前后值、操作人、IP）
- **affiliate_daily_stats 表**：日统计汇总（可选，用于报表加速）

#### API 需求（来自架构文档）

**用户端 API：**
| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 获取推广信息 | GET | /api/v1/affiliate/info | 返回邀请码、链接、收益、档位等 |
| 获取邀请记录 | GET | /api/v1/affiliate/invites | 分页查询邀请列表 |
| 获取佣金记录 | GET | /api/v1/affiliate/commissions | 分页查询佣金列表 |
| 申请提现 | POST | /api/v1/affiliate/withdraw | 提交提现申请 |
| 绑定邀请码 | POST | /api/v1/affiliate/bind | 注册时绑定邀请关系 |

**管理端 API：**
| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 获取配置 | GET | /api/admin/affiliate/config | 获取所有分销配置 |
| 更新配置 | PUT | /api/admin/affiliate/config/:key | 更新指定配置项 |
| 用户列表 | GET | /api/admin/affiliate/users | 查询分销用户列表 |
| 用户详情 | GET | /api/admin/affiliate/users/:id | 查询用户分销详情 |
| 设置 KOL | PUT | /api/admin/affiliate/users/:id/kol | 设置/取消 KOL |
| 提现列表 | GET | /api/admin/affiliate/withdrawals | 查询提现申请列表 |
| 审核通过 | PUT | /api/admin/affiliate/withdrawals/:id/approve | 审核通过 |
| 审核拒绝 | PUT | /api/admin/affiliate/withdrawals/:id/reject | 审核拒绝 |
| 数据概览 | GET | /api/admin/affiliate/stats/overview | 分销数据概览 |
| 日报表 | GET | /api/admin/affiliate/stats/daily | 日统计报表 |
| KOL 排行 | GET | /api/admin/affiliate/stats/kol-ranking | KOL 排行榜 |
| 审计日志 | GET | /api/admin/affiliate/audit-logs | 审计日志查询 |

#### 定时任务需求（来自架构文档）

| 任务名称 | 执行频率 | 说明 |
|---------|---------|------|
| 佣金确认任务 | 每日凌晨 | 确认创建时间超过 7 天且无退款的 pending 佣金 |
| 失败重试任务 | 每 10 分钟 | 重试失败的佣金计算（最多 3 次） |
| 日统计汇总 | 每日凌晨 | 汇总前一天的分销数据（可选） |

#### 缓存策略（来自架构文档）

| Key 格式 | 数据 | TTL | 更新时机 |
|----------|------|-----|----------|
| `aff:config:{key}` | 配置项 | 5 min | 配置更新时删除 |
| `aff:user:{id}` | 用户分销信息 | 10 min | 数据变更时删除 |
| `aff:code:{code}` | code → user_id 映射 | 24 h | 不变 |
| `aff:relation:{invitee_id}` | 邀请关系 | 1 h | 数据变更时删除 |

#### 前端页面需求（来自 UX 文档）

**用户端页面（`src/views/user/affiliate/`）：**
- `AffiliateHomeView.vue` - 推广中心首页（Dashboard）
- `InviteToolsView.vue` - 邀请工具页（链接、海报、二维码）
- `InviteRecordsView.vue` - 邀请记录页
- `CommissionRecordsView.vue` - 佣金明细页
- `WithdrawView.vue` - 提现页
- `WithdrawRecordsView.vue` - 提现记录页
- `AffiliateRulesView.vue` - 规则说明页

**用户端组件（`src/components/user/affiliate/`）：**
- `EarningsCard.vue` - 收益卡片
- `StatsCard.vue` - 战绩卡片
- `TierProgressBar.vue` - 阶梯进度条
- `WithdrawProgressBar.vue` - 提现进度条
- `InviteRecordItem.vue` - 邀请记录项
- `CommissionRecordItem.vue` - 佣金记录项
- `PosterGenerator.vue` - 海报生成器
- `QRCodeDisplay.vue` - 二维码展示

**管理端页面（`src/views/admin/affiliate/`）：**
- `AffiliateConfigView.vue` - 分销配置管理
- `KolManagementView.vue` - KOL 管理
- `WithdrawAuditView.vue` - 提现审核

#### 交互设计需求（来自 UX 文档）

- 复制链接后显示 Toast 提示「复制成功」
- 海报支持 3 套模板选择（简约风、活力风、节日风）
- 进度条可视化（提现进度、阶梯升级进度）
- 阶梯升级时显示成就弹窗
- 支持暗色模式
- 主色使用 Claude Orange (`#d97757`)

---

### FR Coverage Map

| FR | Epic | 简述 |
|----|------|------|
| FR1 | Epic 1 | 生成专属邀请链接 |
| FR2 | Epic 1 | 邀请码自动生成 |
| FR3 | Epic 6 | KOL 自定义推广码 |
| FR4 | Epic 1 | 二维码生成下载 |
| FR5 | Epic 1 | 分享海报模板 |
| FR6 | Epic 1 | 注册时绑定邀请关系 |
| FR7 | Epic 1 | 邀请关系不可更改 |
| FR8 | Epic 1 | 防刷：设备/IP 限制 |
| FR9 | Epic 1 | 邀请人注册奖励 |
| FR10 | Epic 1 | 被邀请人注册奖励 |
| FR11 | Epic 1 | 注册奖励可配置 |
| FR12 | Epic 2 | 首充分成模式 |
| FR13 | Epic 6 | 终身绑定模式（KOL） |
| FR14 | Epic 2 | 分成门槛条件检查 |
| FR15 | Epic 2 | 分成门槛：包月/消费额 |
| FR16 | Epic 4 | 阶梯佣金机制 |
| FR17 | Epic 2 | 佣金计算公式 |
| FR18 | Epic 2 | 有效邀请定义 |
| FR19 | Epic 6 | 设置用户为 KOL |
| FR20 | Epic 6 | KOL 专属推广码 |
| FR21 | Epic 6 | KOL 专属佣金比例 |
| FR22 | Epic 6 | KOL 终身绑定 |
| FR23 | Epic 6 | KOL 关联优惠券 |
| FR24 | Epic 6 | KOL 用户额外注册奖励 |
| FR25 | Epic 6 | KOL 用户首充优惠券 |
| FR26 | Epic 5 | 提现门槛检查 |
| FR27 | Epic 5 | 提现门槛可配置 |
| FR28 | Epic 5 | 支持微信/支付宝 |
| FR29 | Epic 5 | 提现人工审核 |
| FR30 | Epic 5 | T+3 到账 |
| FR31 | Epic 4 | 佣金状态流转 |
| FR32 | Epic 4 | 7 天佣金确认 |
| FR33 | Epic 4 | 退款取消佣金 |
| FR34 | Epic 3 | 查看累计收益 |
| FR35 | Epic 3 | 查看可提现金额 |
| FR36 | Epic 3 | 提现进度条 |
| FR37 | Epic 3 | 查看邀请人数 |
| FR38 | Epic 3 | 查看阶梯档位 |
| FR39 | Epic 3 | 阶梯升级进度条 |
| FR40 | Epic 3 | 一键复制链接 |
| FR41 | Epic 3 | 生成分享海报 |
| FR42 | Epic 3 | 下载二维码 |
| FR43 | Epic 3 | 邀请记录列表 |
| FR44 | Epic 3 | 佣金明细列表 |
| FR45 | Epic 5 | 提现记录列表 |
| FR46 | Epic 3 | 规则说明和 FAQ |
| FR47 | Epic 7 | 配置注册奖励 |
| FR48 | Epic 7 | 配置分成门槛 |
| FR49 | Epic 7 | 配置阶梯档位 |
| FR50 | Epic 7 | 配置佣金上限 |
| FR51 | Epic 7 | 配置提现门槛 |
| FR52 | Epic 7 | 配置确认周期 |
| FR53 | Epic 8 | 查看用户分销数据 |
| FR54 | Epic 8 | 设置/取消 KOL |
| FR55 | Epic 8 | 配置 KOL 推广码 |
| FR56 | Epic 8 | 配置 KOL 佣金比例 |
| FR57 | Epic 8 | 配置 KOL 终身绑定 |
| FR58 | Epic 8 | 配置 KOL 优惠券 |
| FR59 | Epic 8 | 分销概览报表 |
| FR60 | Epic 8 | KOL 排行榜 |
| FR61 | Epic 8 | 提现申请列表 |
| FR62 | Epic 8 | 审核通过提现 |
| FR63 | Epic 8 | 拒绝提现申请 |
| FR64 | Epic 8 | 审计日志查询 |

---

## Epic List

### Epic 1: 邀请系统基础
**用户成果**: 用户可以生成专属邀请链接、二维码和分享海报，新用户通过链接注册后双方获得奖励

**FRs 覆盖:** FR1, FR2, FR4, FR5, FR6, FR7, FR8, FR9, FR10, FR11

**实现要点:**
- 用户注册时自动生成邀请码（6-8位字母数字）
- 邀请链接格式：`{domain}/r/{code}`
- 二维码生成和下载
- 3 套海报模板（简约/活力/节日）
- 注册时绑定邀请关系（不可更改）
- 双向注册奖励发放（使用硬编码默认值）
- 防刷机制（设备/IP 限制）

---

### Epic 2: 首充佣金机制
**用户成果**: 邀请人在被邀请人首次充值时获得佣金分成，满足门槛条件即可享受

**FRs 覆盖:** FR12, FR14, FR15, FR17, FR18

**实现要点:**
- 首充分成模式（默认）
- 分成门槛检查（包月套餐或累计消费）
- 佣金计算：充值金额 × 佣金比例
- 有效邀请定义：被邀请人完成首充
- 佣金记录创建（待确认状态）
- 与充值回调集成

---

### Epic 3: 推广中心用户界面
**用户成果**: 用户可以查看收益数据、邀请记录、佣金明细，了解推广效果和升级进度

**FRs 覆盖:** FR34, FR35, FR36, FR37, FR38, FR39, FR40, FR41, FR42, FR43, FR44, FR46

**实现要点:**
- 推广中心首页（Dashboard）
- 收益卡片：累计收益、可提现金额
- 战绩卡片：邀请人数（总/有效）
- 阶梯进度条和提现进度条
- 一键复制链接（Toast 提示）
- 邀请记录列表（筛选：全部/有效/待激活）
- 佣金明细列表（筛选：全部/待确认/已确认）
- 规则说明和 FAQ 页面

---

### Epic 4: 阶梯佣金与佣金确认
**用户成果**: 邀请人根据有效邀请数自动升级佣金档位，佣金在确认期后自动确认可提现

**FRs 覆盖:** FR16, FR31, FR32, FR33

**实现要点:**
- 阶梯佣金机制（如：0-10人5%，11-30人8%，31+人12%）
- 有效邀请数达标后自动升级档位
- 佣金状态流转：待确认 → 已确认
- 7 天无退款后自动确认（定时任务）
- 退款时自动取消对应佣金
- 阶梯升级成就弹窗

---

### Epic 5: 提现系统
**用户成果**: 用户累计已确认佣金达到门槛后可申请提现，审核通过后到账

**FRs 覆盖:** FR26, FR27, FR28, FR29, FR30, FR45

**实现要点:**
- 提现门槛检查（默认 $100）
- 提现申请表单（金额、方式、账户）
- 支持微信、支付宝
- 提现记录列表
- 并发控制（行锁 + 乐观锁）
- 审核通过后 T+3 到账

---

### Epic 6: KOL 专属功能
**用户成果**: KOL 用户享有自定义推广码、专属佣金比例、终身绑定分成，被邀请用户获得额外优惠

**FRs 覆盖:** FR3, FR13, FR19, FR20, FR21, FR22, FR23, FR24, FR25

**实现要点:**
- KOL 身份标识（is_kol 字段）
- 自定义专属推广码
- 专属佣金比例（不受阶梯限制）
- 终身绑定分成模式
- 关联优惠券模板
- 被邀请用户额外注册奖励
- 被邀请用户首充优惠券

---

### Epic 7: 后台分销配置
**用户成果**: 管理员可以灵活配置所有分销参数，实时调整运营策略

**FRs 覆盖:** FR47, FR48, FR49, FR50, FR51, FR52

**实现要点:**
- 注册奖励金额配置
- 分成门槛条件配置
- 阶梯档位和比例配置
- 佣金比例上限配置
- 提现门槛配置
- 佣金确认周期配置
- 配置热更新（缓存刷新）

---

### Epic 8: 后台用户管理与审核
**用户成果**: 管理员可以管理分销用户、设置 KOL、审核提现申请、查看报表

**FRs 覆盖:** FR53, FR54, FR55, FR56, FR57, FR58, FR59, FR60, FR61, FR62, FR63, FR64

**实现要点:**
- 分销用户列表和搜索
- 用户分销数据详情
- KOL 设置/取消
- KOL 参数配置（推广码、比例、绑定类型、优惠券）
- 提现申请列表
- 审核通过/拒绝（需填写拒绝原因）
- 分销概览报表
- KOL 排行榜
- 审计日志查询

---

## Epic 依赖关系图

```
Epic 1 (邀请系统基础)
    │
    ├──> Epic 2 (首充佣金) ──> Epic 4 (阶梯佣金/确认) ──> Epic 5 (提现)
    │                                │
    │                                └──> Epic 6 (KOL 专属)
    │
    └──> Epic 3 (推广中心 UI) [依赖 Epic 1, 2 数据]

Epic 7 (后台配置) [可与 Epic 1-6 并行]
    │
    └──> Epic 8 (后台管理) [依赖 Epic 5, 6, 7]
```

**建议实施顺序:** Epic 1 → Epic 2 → Epic 3 → Epic 4 → Epic 5 → Epic 6 → Epic 7 → Epic 8

---

## Epic 汇总

| Epic | 名称 | Stories 预估 | FRs 数量 |
|------|------|-------------|----------|
| Epic 1 | 邀请系统基础 | 8-10 | 10 |
| Epic 2 | 首充佣金机制 | 5-6 | 5 |
| Epic 3 | 推广中心用户界面 | 8-10 | 12 |
| Epic 4 | 阶梯佣金与佣金确认 | 4-5 | 4 |
| Epic 5 | 提现系统 | 5-6 | 6 |
| Epic 6 | KOL 专属功能 | 6-8 | 9 |
| Epic 7 | 后台分销配置 | 3-4 | 6 |
| Epic 8 | 后台用户管理与审核 | 8-10 | 12 |
| **总计** | | **47-59** | **64** |

---

## User Stories

### Epic 1: 邀请系统基础

#### Story 1.1: 用户分销信息初始化

**作为** 系统
**我希望** 在用户注册时自动创建分销信息并生成唯一邀请码
**以便** 每个用户都拥有专属的推广身份

**验收标准:**

**Given** 新用户完成注册流程
**When** 用户账户创建成功
**Then** 系统自动在 `user_affiliate` 表中创建对应记录
**And** 生成 6-8 位唯一邀请码（字母数字组合）
**And** 邀请码在数据库中有唯一索引约束
**And** 初始化 tier_level=1, effective_count=0, total_earnings=0, withdrawable=0

**技术要点:**
- 创建 `user_affiliate` 表（Ent schema）
- 邀请码生成算法：随机字母数字，排除易混淆字符（0/O, 1/I/l）
- 在用户注册 Service 中集成调用
- 邀请码唯一性校验，冲突时重新生成

**覆盖需求:** FR2

---

#### Story 1.2: 获取邀请链接和二维码

**作为** 普通用户
**我希望** 获取我的专属邀请链接和二维码图片
**以便** 分享给朋友进行推广

**验收标准:**

**Given** 用户已登录系统
**When** 用户调用获取推广信息接口
**Then** 返回邀请码、邀请链接（格式：`{domain}/r/{code}`）
**And** 返回二维码图片 URL（基于邀请链接生成）
**And** 二维码图片支持下载

**Given** 用户邀请码为 `ABC123`
**When** 生成邀请链接
**Then** 链接格式为 `https://example.com/r/ABC123`

**技术要点:**
- GET `/api/v1/affiliate/info` 接口
- 二维码生成使用 `github.com/skip2/go-qrcode` 库
- 二维码图片可缓存到 CDN
- 响应包含：referral_code, referral_link, qrcode_url

**覆盖需求:** FR1, FR4

---

#### Story 1.3: 生成分享海报

**作为** 普通用户
**我希望** 生成包含二维码的分享海报
**以便** 在社交平台上更有吸引力地推广

**验收标准:**

**Given** 用户已登录且拥有邀请码
**When** 用户选择海报模板并请求生成
**Then** 系统生成包含二维码和利益点文案的海报图片
**And** 提供 3 套模板可选（简约风、活力风、节日风）
**And** 海报图片支持保存到相册

**Given** 用户选择「简约风」模板
**When** 生成海报
**Then** 海报包含：用户二维码、「邀请好友，双方各得 $1」文案、扫码提示

**技术要点:**
- 前端组件：`PosterGenerator.vue`
- 使用 Canvas API 或 html2canvas 生成图片
- 3 套海报模板 JSON 配置（背景、文字位置、样式）
- 二维码动态嵌入

**覆盖需求:** FR5

---

#### Story 1.4: 邀请链接跳转处理

**作为** 潜在用户
**我希望** 点击邀请链接后被引导到注册页面
**以便** 完成注册并绑定邀请关系

**验收标准:**

**Given** 用户点击邀请链接 `{domain}/r/{code}`
**When** 前端解析 URL
**Then** 提取邀请码并存储到 localStorage/Cookie
**And** 跳转到注册页面
**And** 邀请码在 Cookie 中保留 7 天

**Given** 邀请码无效或不存在
**When** 用户访问邀请链接
**Then** 仍然跳转到注册页面（不影响正常注册）
**And** 不存储无效邀请码

**技术要点:**
- 前端路由：`/r/:code` → 注册页
- Cookie 存储：`referral_code`，7 天过期
- 邀请码有效性可选校验（调用后端接口）

**覆盖需求:** FR6（部分）

---

#### Story 1.5: 注册时绑定邀请关系

**作为** 新用户
**我希望** 通过邀请链接注册后自动绑定邀请关系
**以便** 邀请人和我都能获得奖励

**验收标准:**

**Given** 用户通过邀请链接进入注册页面
**When** 用户完成注册
**Then** 系统在 `referral_relation` 表中创建邀请关系记录
**And** 记录包含：inviter_id, invitee_id, referral_code, binding_type=first_charge, invitee_status=registered
**And** 邀请关系创建后不可更改（invitee_id 唯一约束）

**Given** 用户已有邀请关系
**When** 尝试再次绑定邀请码
**Then** 系统拒绝操作并返回「已绑定邀请关系」提示

**Given** 邀请码对应的用户不存在
**When** 用户尝试绑定
**Then** 系统忽略邀请码，正常完成注册（不建立邀请关系）

**技术要点:**
- 创建 `referral_relation` 表（Ent schema）
- POST `/api/v1/affiliate/bind` 接口（注册流程中调用）
- invitee_id 唯一索引确保每个用户只能被邀请一次
- 记录审计日志：`relation_created`

**覆盖需求:** FR6, FR7

---

#### Story 1.6: 发放注册奖励

**作为** 邀请人和被邀请人
**我希望** 邀请关系建立后双方都获得余额奖励
**以便** 激励推广行为

**验收标准:**

**Given** 邀请关系成功建立
**When** 系统处理注册奖励
**Then** 邀请人余额增加配置金额（默认 $1）
**And** 被邀请人余额增加配置金额（默认 $1）
**And** 双方各生成一条 `commission_record`（source_type=register, status=confirmed）
**And** 更新邀请人的 `total_earnings` 和 `withdrawable`

**Given** 奖励配置为邀请人 $1、被邀请人 $0.5
**When** 邀请关系建立
**Then** 邀请人获得 $1，被邀请人获得 $0.5

**技术要点:**
- 创建 `commission_record` 表（Ent schema）
- 注册奖励使用硬编码默认值（Epic 7 实现配置化）
- 在同一事务中：创建关系 + 增加余额 + 创建佣金记录
- 注册奖励直接确认（status=confirmed），无需等待确认期

**覆盖需求:** FR9, FR10, FR11（部分，使用默认值）

---

#### Story 1.7: 注册防刷机制

**作为** 系统
**我希望** 限制同一设备/IP 短时间内的注册数量
**以便** 防止羊毛党批量注册薅奖励

**验收标准:**

**Given** 同一 IP 地址在 24 小时内已注册 5 个账号
**When** 该 IP 尝试再次注册
**Then** 系统拒绝注册并返回「注册过于频繁，请稍后再试」

**Given** 同一设备指纹在 24 小时内已注册 3 个账号
**When** 该设备尝试再次注册
**Then** 系统拒绝注册并提示限制信息

**Given** 触发防刷限制
**When** 24 小时后
**Then** 限制自动解除，可以正常注册

**技术要点:**
- Redis 计数器：`register:ip:{ip}:daily`、`register:device:{fingerprint}:daily`
- IP 限制：24h 内最多 5 次
- 设备指纹限制：24h 内最多 3 次
- 前端传递设备指纹（使用 fingerprintjs 或类似库）
- 限制值可配置（后续 Epic 7 实现）

**覆盖需求:** FR8, NFR1

---

### Epic 1 完成汇总

| Story | 标题 | 覆盖 FR |
|-------|------|---------|
| 1.1 | 用户分销信息初始化 | FR2 |
| 1.2 | 获取邀请链接和二维码 | FR1, FR4 |
| 1.3 | 生成分享海报 | FR5 |
| 1.4 | 邀请链接跳转处理 | FR6（部分） |
| 1.5 | 注册时绑定邀请关系 | FR6, FR7 |
| 1.6 | 发放注册奖励 | FR9, FR10, FR11 |
| 1.7 | 注册防刷机制 | FR8 |

---

### Epic 2: 首充佣金机制

#### Story 2.1: 查询邀请关系

**作为** 系统
**我希望** 在被邀请人充值时查询其邀请关系
**以便** 确定是否需要计算佣金

**验收标准:**

**Given** 被邀请人发起充值
**When** 充值成功回调触发
**Then** 系统查询该用户的邀请关系（invitee_id）
**And** 如果存在邀请关系，返回邀请人信息和绑定类型
**And** 如果不存在邀请关系，跳过佣金计算

**技术要点:**
- `AffiliateService.GetRelationByInvitee(userId)` 方法
- Redis 缓存邀请关系：`aff:relation:{invitee_id}`，TTL 1h
- 缓存未命中时查询数据库并回填

**覆盖需求:** FR12（前置）

---

#### Story 2.2: 分成门槛检查

**作为** 系统
**我希望** 检查邀请人是否满足分成门槛条件
**以便** 只有符合条件的邀请人才能获得佣金

**验收标准:**

**Given** 存在有效邀请关系
**When** 系统检查邀请人的分成资格
**Then** 检查条件1：邀请人是否购买过包月套餐
**And** 检查条件2：邀请人历史消费是否累计满配置金额（默认 $10）
**And** 两个条件满足其一即可获得分成资格

**Given** 邀请人未购买包月且消费不足 $10
**When** 被邀请人充值
**Then** 邀请人不获得佣金（但邀请关系保留）

**技术要点:**
- `AffiliateService.CheckThreshold(inviterId)` 方法
- 查询用户订阅状态和累计消费
- 门槛值使用硬编码默认值（Epic 7 实现配置化）

**覆盖需求:** FR14, FR15

---

#### Story 2.3: 首充佣金计算

**作为** 邀请人
**我希望** 在被邀请人首次充值时获得佣金
**以便** 从推广中获得收益

**验收标准:**

**Given** 被邀请人首次充值成功
**And** 邀请人满足分成门槛
**And** 绑定类型为 `first_charge`
**When** 系统计算佣金
**Then** 佣金 = 充值金额 × 邀请人当前佣金比例（默认 5%）
**And** 创建佣金记录（status=pending）
**And** 更新被邀请人状态为 `first_charged`
**And** 更新邀请人的 `effective_count` +1

**Given** 被邀请人已有充值记录（非首充）
**And** 绑定类型为 `first_charge`
**When** 被邀请人再次充值
**Then** 不计算佣金（首充模式只计算一次）

**技术要点:**
- `AffiliateService.CalculateCommission(order)` 方法
- 在充值回调中异步调用（goroutine）
- 使用数据库事务保证原子性
- 订单号唯一索引保证幂等性
- 记录审计日志：`commission_created`

**覆盖需求:** FR12, FR17, FR18

---

#### Story 2.4: 佣金计算失败重试

**作为** 系统
**我希望** 佣金计算失败时记录并重试
**以便** 保证佣金不丢失

**验收标准:**

**Given** 佣金计算过程中发生异常
**When** 异常被捕获
**Then** 在 `commission_retry` 表中记录失败信息
**And** 不影响充值主流程（充值仍然成功）
**And** 记录详细错误日志

**Given** 存在待重试的佣金记录
**When** 定时任务执行（每 10 分钟）
**Then** 重新尝试计算佣金
**And** 最多重试 3 次
**And** 超过 3 次标记为 failed（人工处理）

**技术要点:**
- 创建 `commission_retry` 表
- 定时任务：`CommissionRetryJob`
- 告警：重试队列堆积 > 100 条

**覆盖需求:** NFR16, NFR17

---

### Epic 2 完成汇总

| Story | 标题 | 覆盖 FR/NFR |
|-------|------|-------------|
| 2.1 | 查询邀请关系 | FR12（前置） |
| 2.2 | 分成门槛检查 | FR14, FR15 |
| 2.3 | 首充佣金计算 | FR12, FR17, FR18 |
| 2.4 | 佣金计算失败重试 | NFR16, NFR17 |

---

### Epic 3: 推广中心用户界面

#### Story 3.1: 推广中心首页 - 收益卡片

**作为** 普通用户
**我希望** 在推广中心看到我的收益概览
**以便** 快速了解推广效果

**验收标准:**

**Given** 用户进入推广中心首页
**When** 页面加载完成
**Then** 显示累计收益金额
**And** 显示可提现金额
**And** 显示提现进度条（距离 $100 门槛的百分比）
**And** 可提现金额 < $100 时显示「还差 $XX」

**技术要点:**
- GET `/api/v1/affiliate/info` 接口扩展
- 前端页面：`AffiliateHomeView.vue`
- 前端组件：`EarningsCard.vue`、`WithdrawProgressBar.vue`

**覆盖需求:** FR34, FR35, FR36

---

#### Story 3.2: 推广中心首页 - 战绩卡片

**作为** 普通用户
**我希望** 查看我的邀请数据和阶梯进度
**以便** 了解当前档位和升级目标

**验收标准:**

**Given** 用户进入推广中心首页
**When** 页面加载完成
**Then** 显示邀请用户总数
**And** 显示有效用户数（已首充）
**And** 显示当前阶梯档位名称和佣金比例
**And** 显示阶梯升级进度条（再邀请 N 人升级）

**Given** 用户当前档位为「白银」（11-30人，8%）
**And** 有效邀请数为 18
**When** 查看战绩卡片
**Then** 显示「当前档位：白银 ⭐⭐」「佣金比例：8%」
**And** 进度条显示「再邀请 12 人升级黄金档位 (12%)」

**技术要点:**
- 前端组件：`StatsCard.vue`、`TierProgressBar.vue`
- API 返回：tier_level, tier_name, commission_rate, effective_count, next_tier_threshold

**覆盖需求:** FR37, FR38, FR39

---

#### Story 3.3: 一键复制邀请链接

**作为** 普通用户
**我希望** 一键复制我的邀请链接
**以便** 快速分享给朋友

**验收标准:**

**Given** 用户在推广中心
**When** 点击「复制链接」按钮
**Then** 邀请链接复制到剪贴板
**And** 显示 Toast 提示「复制成功」
**And** Toast 2 秒后自动消失

**技术要点:**
- 使用 Clipboard API
- Toast 组件显示

**覆盖需求:** FR40

---

#### Story 3.4: 邀请记录列表

**作为** 普通用户
**我希望** 查看我邀请的用户列表
**以便** 了解邀请详情和贡献情况

**验收标准:**

**Given** 用户进入邀请记录页面
**When** 页面加载
**Then** 显示邀请用户列表（分页，每页 20 条）
**And** 每条记录显示：脱敏昵称、注册时间、状态、首充时间、累计贡献

**Given** 用户点击筛选 Tab
**When** 选择「有效」Tab
**Then** 只显示已首充的用户

**Given** 用户点击筛选 Tab
**When** 选择「待激活」Tab
**Then** 只显示未首充的用户

**技术要点:**
- GET `/api/v1/affiliate/invites?page=1&size=20&status=all|effective|pending`
- 前端页面：`InviteRecordsView.vue`
- 前端组件：`InviteRecordItem.vue`

**覆盖需求:** FR43

---

#### Story 3.5: 佣金明细列表

**作为** 普通用户
**我希望** 查看我的佣金明细
**以便** 了解每笔收益的来源和状态

**验收标准:**

**Given** 用户进入佣金明细页面
**When** 页面加载
**Then** 显示佣金记录列表（分页，每页 20 条）
**And** 每条记录显示：类型（注册/充值）、来源用户、金额、比例、状态、时间
**And** 按时间倒序排列，按月份分组显示

**Given** 佣金状态为「待确认」
**When** 显示记录
**Then** 显示「待确认（还剩 N 天）」

**技术要点:**
- GET `/api/v1/affiliate/commissions?page=1&size=20&status=all|pending|confirmed`
- 前端页面：`CommissionRecordsView.vue`
- 前端组件：`CommissionRecordItem.vue`

**覆盖需求:** FR44

---

#### Story 3.6: 规则说明页面

**作为** 普通用户
**我希望** 查看推广规则详情
**以便** 了解奖励机制和提现规则

**验收标准:**

**Given** 用户点击「查看规则」
**When** 进入规则说明页面
**Then** 显示邀请奖励规则
**And** 显示阶梯佣金比例表
**And** 显示分成门槛条件
**And** 显示提现规则（门槛、周期、方式）
**And** 显示常见问题 FAQ（可折叠）

**技术要点:**
- 前端页面：`AffiliateRulesView.vue`
- 规则内容可硬编码或从配置获取

**覆盖需求:** FR46

---

### Epic 3 完成汇总

| Story | 标题 | 覆盖 FR |
|-------|------|---------|
| 3.1 | 推广中心首页 - 收益卡片 | FR34, FR35, FR36 |
| 3.2 | 推广中心首页 - 战绩卡片 | FR37, FR38, FR39 |
| 3.3 | 一键复制邀请链接 | FR40 |
| 3.4 | 邀请记录列表 | FR43 |
| 3.5 | 佣金明细列表 | FR44 |
| 3.6 | 规则说明页面 | FR46 |

**注意:** FR41（生成海报）和 FR42（下载二维码）已在 Epic 1 Story 1.2/1.3 中实现

---

### Epic 4: 阶梯佣金与佣金确认

#### Story 4.1: 阶梯佣金比例计算

**作为** 邀请人
**我希望** 根据有效邀请数自动获得更高的佣金比例
**以便** 激励我邀请更多用户

**验收标准:**

**Given** 邀请人有效邀请数为 0-10 人
**When** 计算佣金比例
**Then** 使用档位1比例（默认 5%）

**Given** 邀请人有效邀请数为 11-30 人
**When** 计算佣金比例
**Then** 使用档位2比例（默认 8%）

**Given** 邀请人有效邀请数为 31+ 人
**When** 计算佣金比例
**Then** 使用档位3比例（默认 12%）

**技术要点:**
- `AffiliateService.GetCommissionRate(inviter)` 方法
- 阶梯规则使用硬编码默认值（Epic 7 实现配置化）
- 在 Story 2.3 佣金计算中调用

**覆盖需求:** FR16

---

#### Story 4.2: 阶梯自动升级

**作为** 邀请人
**我希望** 有效邀请数达标后自动升级档位
**以便** 后续佣金使用更高比例计算

**验收标准:**

**Given** 邀请人当前档位为1（0-10人）
**And** 有效邀请数增加到 11
**When** 系统更新有效邀请数
**Then** 自动将 tier_level 更新为 2
**And** 记录审计日志：`tier_upgraded`

**Given** 邀请人档位升级
**When** 前端下次加载推广中心
**Then** 显示升级成就弹窗「恭喜升级为白银推广员」

**技术要点:**
- `AffiliateService.UpdateTierLevel(tx, userId)` 方法
- 在 Story 2.3 有效邀请数更新后调用
- 前端检测 tier_level 变化显示弹窗

**覆盖需求:** FR16（升级逻辑）

---

#### Story 4.3: 佣金自动确认定时任务

**作为** 系统
**我希望** 定时确认超过确认期的待确认佣金
**以便** 佣金可以提现

**验收标准:**

**Given** 存在 pending 状态的佣金记录
**And** 创建时间超过 7 天
**And** 关联订单未退款
**When** 定时任务执行（每日凌晨）
**Then** 将佣金状态更新为 confirmed
**And** 更新 confirmed_at 时间
**And** 增加邀请人的 withdrawable 金额
**And** 记录审计日志：`commission_confirmed`

**Given** 佣金关联的订单已退款
**When** 定时任务执行
**Then** 将佣金状态更新为 cancelled
**And** 记录审计日志：`commission_cancelled`

**技术要点:**
- 定时任务：`CommissionConfirmJob`，cron: `0 2 * * *`
- 批量处理，每次最多 1000 条
- 需要检查关联订单的退款状态

**覆盖需求:** FR31, FR32

---

#### Story 4.4: 退款取消佣金

**作为** 系统
**我希望** 充值订单退款时自动取消对应佣金
**以便** 防止虚假充值套利

**验收标准:**

**Given** 充值订单发生退款
**When** 退款回调触发
**Then** 查询该订单关联的佣金记录
**And** 将佣金状态更新为 cancelled
**And** 如果佣金已确认，从 withdrawable 中扣减
**And** 记录审计日志：`commission_cancelled`

**Given** 订单对应的佣金已被提现
**When** 订单退款
**Then** 记录警告日志（需人工处理）

**技术要点:**
- 在退款回调中调用 `AffiliateService.CancelCommission(orderId)`
- 使用数据库事务保证一致性

**覆盖需求:** FR33

---

### Epic 4 完成汇总

| Story | 标题 | 覆盖 FR |
|-------|------|---------|
| 4.1 | 阶梯佣金比例计算 | FR16 |
| 4.2 | 阶梯自动升级 | FR16 |
| 4.3 | 佣金自动确认定时任务 | FR31, FR32 |
| 4.4 | 退款取消佣金 | FR33 |

---

### Epic 5: 提现系统

#### Story 5.1: 提现资格检查

**作为** 普通用户
**我希望** 系统检查我是否满足提现条件
**以便** 了解何时可以提现

**验收标准:**

**Given** 用户可提现金额 >= $100（门槛）
**When** 查询提现资格
**Then** 返回 can_withdraw=true

**Given** 用户可提现金额 < $100
**When** 查询提现资格
**Then** 返回 can_withdraw=false
**And** 返回距离门槛还差多少金额

**技术要点:**
- 在 `/api/v1/affiliate/info` 响应中包含 can_withdraw 字段
- 提现门槛使用硬编码默认值（Epic 7 实现配置化）

**覆盖需求:** FR26, FR27

---

#### Story 5.2: 提现申请提交

**作为** 普通用户
**我希望** 提交提现申请
**以便** 将佣金转入我的账户

**验收标准:**

**Given** 用户满足提现条件
**When** 填写提现金额、方式、账户并提交
**Then** 创建提现记录（status=pending）
**And** 扣减用户的 withdrawable 金额（使用行锁+乐观锁）
**And** 返回提现申请 ID 和预计到账时间
**And** 记录审计日志：`withdraw_requested`

**Given** 提现金额 > 可提现金额
**When** 提交申请
**Then** 返回错误「提现金额超过可提现余额」

**Given** 提现金额 < 门槛
**When** 提交申请
**Then** 返回错误「未达到最低提现金额」

**技术要点:**
- 创建 `withdrawal_record` 表
- POST `/api/v1/affiliate/withdraw`
- 并发控制：SELECT FOR UPDATE + version 乐观锁
- 加密存储收款账户信息

**覆盖需求:** FR26, FR28, NFR10

---

#### Story 5.3: 提现方式选择

**作为** 普通用户
**我希望** 选择微信或支付宝作为提现方式
**以便** 将佣金转入我常用的账户

**验收标准:**

**Given** 用户进入提现页面
**When** 选择提现方式
**Then** 可选择「微信」或「支付宝」
**And** 填写对应的收款账户信息

**Given** 选择微信
**When** 填写账户
**Then** 输入微信号或手机号

**Given** 选择支付宝
**When** 填写账户
**Then** 输入支付宝账号（手机号或邮箱）

**技术要点:**
- 前端页面：`WithdrawView.vue`
- 账户信息脱敏显示：`138****8888`

**覆盖需求:** FR28

---

#### Story 5.4: 提现记录列表

**作为** 普通用户
**我希望** 查看我的提现记录
**以便** 了解提现进度和历史

**验收标准:**

**Given** 用户进入提现记录页面
**When** 页面加载
**Then** 显示提现记录列表（分页）
**And** 每条记录显示：金额、方式、脱敏账户、申请时间、状态、完成时间
**And** 按时间倒序排列

**Given** 提现状态为「审核中」
**When** 显示记录
**Then** 状态显示为黄色「审核中」图标

**Given** 提现状态为「已完成」
**When** 显示记录
**Then** 状态显示为绿色「已完成」图标

**技术要点:**
- GET `/api/v1/affiliate/withdrawals?page=1&size=20`
- 前端页面：`WithdrawRecordsView.vue`

**覆盖需求:** FR45

---

### Epic 5 完成汇总

| Story | 标题 | 覆盖 FR/NFR |
|-------|------|-------------|
| 5.1 | 提现资格检查 | FR26, FR27 |
| 5.2 | 提现申请提交 | FR26, FR28, NFR10 |
| 5.3 | 提现方式选择 | FR28 |
| 5.4 | 提现记录列表 | FR45 |

**注意:** FR29（人工审核）和 FR30（T+3 到账）在 Epic 8 后台审核中实现

---

### Epic 6: KOL 专属功能

#### Story 6.1: KOL 身份标识

**作为** KOL 用户
**我希望** 系统识别我的 KOL 身份
**以便** 享受专属功能

**验收标准:**

**Given** 用户被管理员设置为 KOL
**When** 查询用户分销信息
**Then** is_kol=true
**And** kol_config 包含专属配置

**Given** 普通用户
**When** 查询用户分销信息
**Then** is_kol=false
**And** kol_config 为空

**技术要点:**
- `user_affiliate` 表 is_kol 字段
- `kol_config` JSONB 字段存储专属配置

**覆盖需求:** FR19（数据层）

---

#### Story 6.2: KOL 自定义推广码

**作为** KOL 用户
**我希望** 使用我的专属推广码
**以便** 建立个人品牌

**验收标准:**

**Given** 用户是 KOL
**And** 管理员为其设置了专属推广码「DAXIN」
**When** 查询推广信息
**Then** 返回专属推广码和对应链接 `{domain}/r/DAXIN`

**Given** 潜在用户使用专属推广码注册
**When** 绑定邀请关系
**Then** 使用专属推广码查找对应 KOL
**And** 建立邀请关系

**技术要点:**
- `kol_config.promo_code` 字段
- 邀请码查询逻辑：先查 user_affiliate.referral_code，再查 kol_config.promo_code
- Redis 缓存：`aff:code:{code}` 支持 KOL 码

**覆盖需求:** FR3, FR20

---

#### Story 6.3: KOL 专属佣金比例

**作为** KOL 用户
**我希望** 使用我的专属佣金比例
**以便** 获得更高收益

**验收标准:**

**Given** KOL 配置了专属佣金比例 10%
**When** 被邀请人充值
**Then** 使用 10% 计算佣金（不受阶梯限制）

**Given** KOL 未配置专属比例
**When** 被邀请人充值
**Then** 使用阶梯佣金比例

**技术要点:**
- `kol_config.commission_rate` 字段
- 在 `GetCommissionRate()` 中优先使用 KOL 专属比例

**覆盖需求:** FR21

---

#### Story 6.4: KOL 终身绑定分成

**作为** KOL 用户
**我希望** 被邀请人每次充值我都能获得佣金
**以便** 持续获得收益

**验收标准:**

**Given** KOL 开启了终身绑定
**And** 被邀请人已首充
**When** 被邀请人再次充值
**Then** KOL 仍然获得佣金

**Given** 普通用户（首充分成模式）
**And** 被邀请人已首充
**When** 被邀请人再次充值
**Then** 不产生佣金

**技术要点:**
- `kol_config.default_binding_type` 字段
- `referral_relation.binding_type` 存储实际绑定类型
- 在佣金计算中检查 binding_type

**覆盖需求:** FR13, FR22

---

#### Story 6.5: KOL 用户专属奖励

**作为** 通过 KOL 推广码注册的用户
**我希望** 获得额外的注册奖励
**以便** 享受 KOL 专属福利

**验收标准:**

**Given** 用户通过 KOL 推广码注册
**And** KOL 配置了额外注册奖励 $2
**When** 邀请关系建立
**Then** 被邀请人获得普通奖励 + 额外奖励（共 $3）

**技术要点:**
- `kol_config.user_bonus` 字段
- 在 Story 1.6 注册奖励逻辑中检查 KOL 配置

**覆盖需求:** FR24

---

#### Story 6.6: KOL 优惠券关联（预留）

**作为** 通过 KOL 推广码注册的用户
**我希望** 获得首充优惠券
**以便** 享受充值优惠

**验收标准:**

**Given** KOL 关联了优惠券模板
**When** 用户通过 KOL 推广码注册
**Then** 自动发放关联的优惠券给用户

**技术要点:**
- `kol_config.coupon_template_id` 字段
- 集成现有优惠券系统（如有）
- 如无优惠券系统，此 Story 标记为预留

**覆盖需求:** FR23, FR25

---

### Epic 6 完成汇总

| Story | 标题 | 覆盖 FR |
|-------|------|---------|
| 6.1 | KOL 身份标识 | FR19 |
| 6.2 | KOL 自定义推广码 | FR3, FR20 |
| 6.3 | KOL 专属佣金比例 | FR21 |
| 6.4 | KOL 终身绑定分成 | FR13, FR22 |
| 6.5 | KOL 用户专属奖励 | FR24 |
| 6.6 | KOL 优惠券关联（预留） | FR23, FR25 |

---

### Epic 7: 后台分销配置

#### Story 7.1: 分销配置数据表

**作为** 系统
**我希望** 将分销配置存储到数据库
**以便** 支持动态调整

**验收标准:**

**Given** 系统启动
**When** 初始化配置
**Then** `affiliate_config` 表中存在默认配置项
**And** 包含：register_bonus, commission_threshold, tier_rules, withdraw_threshold, commission_confirm_days

**技术要点:**
- 创建 `affiliate_config` 表
- 配置项使用 JSONB 存储值
- 初始化脚本插入默认值

**覆盖需求:** 数据层准备

---

#### Story 7.2: 后台配置管理页面

**作为** 管理员
**我希望** 在后台配置分销参数
**以便** 灵活调整运营策略

**验收标准:**

**Given** 管理员进入分销配置页面
**When** 页面加载
**Then** 显示所有配置项当前值
**And** 可编辑：注册奖励（邀请人/被邀请人）、分成门槛、阶梯档位、提现门槛、确认周期

**Given** 管理员修改配置并保存
**When** 保存成功
**Then** 配置存入数据库
**And** 清除相关缓存
**And** 显示保存成功提示

**技术要点:**
- GET `/api/admin/affiliate/config`
- PUT `/api/admin/affiliate/config/:key`
- 前端页面：`AffiliateConfigView.vue`
- 配置更新时删除 Redis 缓存

**覆盖需求:** FR47, FR48, FR49, FR50, FR51, FR52

---

#### Story 7.3: 配置热更新

**作为** 系统
**我希望** 配置修改后立即生效
**以便** 无需重启服务

**验收标准:**

**Given** 管理员修改配置
**When** 保存成功
**Then** 清除 Redis 中对应配置缓存
**And** 下次读取时从数据库加载最新值
**And** 业务逻辑使用新配置值

**技术要点:**
- `AffiliateConfigService.InvalidateCache(key)`
- 配置读取：先查缓存，未命中查数据库
- 缓存 TTL: 5 分钟

**覆盖需求:** 配置实时生效

---

### Epic 7 完成汇总

| Story | 标题 | 覆盖 FR |
|-------|------|---------|
| 7.1 | 分销配置数据表 | 数据层 |
| 7.2 | 后台配置管理页面 | FR47-52 |
| 7.3 | 配置热更新 | 实时生效 |

---

### Epic 8: 后台用户管理与审核

#### Story 8.1: 分销用户列表

**作为** 管理员
**我希望** 查看分销用户列表
**以便** 了解整体分销情况

**验收标准:**

**Given** 管理员进入分销用户管理页面
**When** 页面加载
**Then** 显示用户列表（分页）
**And** 每条记录显示：用户ID、昵称、邀请码、档位、有效邀请数、累计收益、是否KOL

**Given** 管理员输入搜索条件
**When** 搜索用户ID或昵称
**Then** 返回匹配的用户列表

**技术要点:**
- GET `/api/admin/affiliate/users?keyword=&is_kol=&page=1`
- 前端页面：`KolManagementView.vue`（复用）

**覆盖需求:** FR53

---

#### Story 8.2: 设置 KOL

**作为** 管理员
**我希望** 将用户设置为 KOL
**以便** 开展 KOL 合作推广

**验收标准:**

**Given** 管理员查看用户详情
**When** 点击「设置为 KOL」
**Then** 显示 KOL 配置表单
**And** 可配置：专属推广码、佣金比例、绑定类型、额外奖励

**Given** 管理员填写配置并保存
**When** 保存成功
**Then** 用户 is_kol=true，kol_config 存储配置
**And** 记录审计日志：`kol_granted`

**Given** 管理员取消 KOL 身份
**When** 点击「取消 KOL」
**Then** is_kol=false，保留 kol_config（便于恢复）
**And** 记录审计日志：`kol_revoked`

**技术要点:**
- PUT `/api/admin/affiliate/users/:id/kol`
- 创建 `affiliate_audit_log` 表

**覆盖需求:** FR54, FR55, FR56, FR57, FR58

---

#### Story 8.3: 提现审核列表

**作为** 管理员
**我希望** 查看待审核的提现申请
**以便** 处理用户提现

**验收标准:**

**Given** 管理员进入提现审核页面
**When** 页面加载
**Then** 显示提现申请列表（默认显示待审核）
**And** 每条记录显示：用户、金额、方式、账户、申请时间

**Given** 管理员点击查看详情
**When** 详情弹窗打开
**Then** 显示用户分销数据（累计收益、已提现、有效邀请数）

**技术要点:**
- GET `/api/admin/affiliate/withdrawals?status=pending&page=1`
- 前端页面：`WithdrawAuditView.vue`

**覆盖需求:** FR61

---

#### Story 8.4: 审核通过提现

**作为** 管理员
**我希望** 审核通过提现申请
**以便** 用户可以收到款项

**验收标准:**

**Given** 管理员查看提现详情
**When** 点击「通过并打款」
**Then** 提现状态更新为 approved
**And** 记录审核人和审核时间
**And** 记录审计日志：`withdraw_approved`

**Given** 提现审核通过后 T+3
**When** 打款完成
**Then** 状态更新为 completed
**And** 记录完成时间
**And** 记录审计日志：`withdraw_completed`

**技术要点:**
- PUT `/api/admin/affiliate/withdrawals/:id/approve`
- 实际打款流程可能是线下操作

**覆盖需求:** FR29, FR30, FR62

---

#### Story 8.5: 审核拒绝提现

**作为** 管理员
**我希望** 拒绝不合规的提现申请
**以便** 保护平台利益

**验收标准:**

**Given** 管理员查看提现详情
**When** 填写拒绝原因并点击「拒绝」
**Then** 提现状态更新为 rejected
**And** 记录拒绝原因、审核人、审核时间
**And** 将金额退回用户的 withdrawable
**And** 记录审计日志：`withdraw_rejected`

**技术要点:**
- PUT `/api/admin/affiliate/withdrawals/:id/reject`
- 需要在事务中退回金额

**覆盖需求:** FR63

---

#### Story 8.6: 分销数据报表

**作为** 管理员
**我希望** 查看分销数据概览
**以便** 了解分销业务整体情况

**验收标准:**

**Given** 管理员进入数据报表页面
**When** 页面加载
**Then** 显示概览数据：总邀请数、有效邀请数、佣金总额、提现总额
**And** 显示日趋势图（最近 30 天）

**Given** 管理员选择日期范围
**When** 查询日报表
**Then** 显示每日统计数据

**技术要点:**
- GET `/api/admin/affiliate/stats/overview`
- GET `/api/admin/affiliate/stats/daily?start=&end=`
- 可使用 `affiliate_daily_stats` 表加速查询

**覆盖需求:** FR59

---

#### Story 8.7: KOL 排行榜

**作为** 管理员
**我希望** 查看 KOL 业绩排行
**以便** 评估 KOL 合作效果

**验收标准:**

**Given** 管理员进入 KOL 排行页面
**When** 页面加载
**Then** 显示 KOL 排行榜（按有效邀请数或佣金排序）
**And** 每条记录显示：KOL 昵称、推广码、有效邀请数、佣金总额

**技术要点:**
- GET `/api/admin/affiliate/stats/kol-ranking?sort_by=effective_count|earnings`

**覆盖需求:** FR60

---

#### Story 8.8: 审计日志查询

**作为** 管理员
**我希望** 查询分销相关的操作日志
**以便** 追溯问题和审计

**验收标准:**

**Given** 管理员进入审计日志页面
**When** 查询指定用户或操作类型
**Then** 返回匹配的日志列表
**And** 每条记录显示：操作类型、用户、变更前后值、操作人、时间

**技术要点:**
- GET `/api/admin/affiliate/audit-logs?user_id=&action=&page=1`
- `affiliate_audit_log` 表查询

**覆盖需求:** FR64, NFR6

---

### Epic 8 完成汇总

| Story | 标题 | 覆盖 FR/NFR |
|-------|------|-------------|
| 8.1 | 分销用户列表 | FR53 |
| 8.2 | 设置 KOL | FR54-58 |
| 8.3 | 提现审核列表 | FR61 |
| 8.4 | 审核通过提现 | FR29, FR30, FR62 |
| 8.5 | 审核拒绝提现 | FR63 |
| 8.6 | 分销数据报表 | FR59 |
| 8.7 | KOL 排行榜 | FR60 |
| 8.8 | 审计日志查询 | FR64, NFR6 |

---

## Stories 汇总

| Epic | 名称 | Stories 数量 |
|------|------|-------------|
| Epic 1 | 邀请系统基础 | 7 |
| Epic 2 | 首充佣金机制 | 4 |
| Epic 3 | 推广中心用户界面 | 6 |
| Epic 4 | 阶梯佣金与佣金确认 | 4 |
| Epic 5 | 提现系统 | 4 |
| Epic 6 | KOL 专属功能 | 6 |
| Epic 7 | 后台分销配置 | 3 |
| Epic 8 | 后台用户管理与审核 | 8 |
| **总计** | | **42** |

---

## Implementation Priority

### Sprint 1: 基础邀请 + 注册奖励 (MVP)
**目标**: 用户可以生成邀请链接，新用户注册双方获得奖励

1. Story 1.1 - 用户分销信息初始化
2. Story 1.2 - 获取邀请链接和二维码
3. Story 1.4 - 邀请链接跳转处理
4. Story 1.5 - 注册时绑定邀请关系
5. Story 1.6 - 发放注册奖励
6. Story 1.7 - 注册防刷机制

### Sprint 2: 首充佣金 + 基础 UI
**目标**: 邀请人可以获得首充佣金，查看推广数据

7. Story 2.1 - 查询邀请关系
8. Story 2.2 - 分成门槛检查
9. Story 2.3 - 首充佣金计算
10. Story 3.1 - 推广中心首页 - 收益卡片
11. Story 3.2 - 推广中心首页 - 战绩卡片
12. Story 3.3 - 一键复制邀请链接
13. Story 3.4 - 邀请记录列表

### Sprint 3: 阶梯佣金 + 完整 UI
**目标**: 完整的阶梯佣金机制和用户界面

14. Story 4.1 - 阶梯佣金比例计算
15. Story 4.2 - 阶梯自动升级
16. Story 4.3 - 佣金自动确认定时任务
17. Story 4.4 - 退款取消佣金
18. Story 3.5 - 佣金明细列表
19. Story 3.6 - 规则说明页面
20. Story 1.3 - 生成分享海报

### Sprint 4: 提现系统
**目标**: 用户可以申请提现

21. Story 5.1 - 提现资格检查
22. Story 5.2 - 提现申请提交
23. Story 5.3 - 提现方式选择
24. Story 5.4 - 提现记录列表
25. Story 2.4 - 佣金计算失败重试

### Sprint 5: 后台管理
**目标**: 管理员可以配置参数和审核提现

26. Story 7.1 - 分销配置数据表
27. Story 7.2 - 后台配置管理页面
28. Story 7.3 - 配置热更新
29. Story 8.1 - 分销用户列表
30. Story 8.3 - 提现审核列表
31. Story 8.4 - 审核通过提现
32. Story 8.5 - 审核拒绝提现

### Sprint 6: KOL 体系
**目标**: 完整的 KOL 功能

33. Story 6.1 - KOL 身份标识
34. Story 6.2 - KOL 自定义推广码
35. Story 6.3 - KOL 专属佣金比例
36. Story 6.4 - KOL 终身绑定分成
37. Story 6.5 - KOL 用户专属奖励
38. Story 8.2 - 设置 KOL

### Sprint 7: 报表与优化
**目标**: 数据报表和系统优化

39. Story 8.6 - 分销数据报表
40. Story 8.7 - KOL 排行榜
41. Story 8.8 - 审计日志查询
42. Story 6.6 - KOL 优惠券关联（预留）
