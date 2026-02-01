# Story 7.4: 退款状态更新与日志

Status: done

## Story

**作为** 系统
**我希望** 更新订单状态为已退款并记录退款日志
**以便** 完整追踪退款流程

## Acceptance Criteria

- [x] AC1: 更新订单状态为 `refunded`
- [x] AC2: 记录退款时间、退款金额、退款原因
- [x] AC3: 记录操作人（admin_id）
- [x] AC4: 插入退款日志到 `balance_logs` 表
- [x] AC5: 日志包含退款前后余额

## Tasks / Subtasks

- [x] Task 1: 后端 - 订单表字段扩展 (AC: 1, 2, 3)
  - [x] 1.1 添加 refund_no 字段（退款单号）- Story 7-2 已完成
  - [x] 1.2 添加 refund_status 字段（退款状态）- Story 7-2 已完成
  - [x] 1.3 添加 refunded_at 字段（退款时间）- Story 7-2 已完成
  - [x] 1.4 添加 refund_reason 字段（退款原因）- Story 7-2 已完成
  - [x] 1.5 添加 refund_admin_id 字段（操作人ID）- Story 7-2 已完成
  - [x] 1.6 运行数据库迁移 - Story 7-2 已完成

- [x] Task 2: 后端 - 订单状态更新 (AC: 1, 2, 3)
  - [x] 2.1 在退款成功处理中更新订单状态 - Story 7-3 已完成
  - [x] 2.2 记录所有退款相关字段 - Story 7-3 已完成

- [x] Task 3: 后端 - 余额日志记录 (AC: 4, 5)
  - [x] 3.1 确保 balance_logs 记录完整的退款信息 - Story 7-3 已完成
  - [x] 3.2 记录 change_type = refund - Story 7-3 已完成
  - [x] 3.3 记录操作人信息 - Story 7-3 已完成

- [x] Task 4: 后端 - 退款历史查询 (AC: 1-5)
  - [x] 4.1 管理端可查询退款订单列表（通过 status=refunded 筛选）
  - [x] 4.2 管理端可查询订单余额日志（新增 GetOrderLogs API）

- [x] Task 5: 数据库迁移 (AC: 1)
  - [x] 5.1 生成 Ent 迁移文件 - Story 7-2 已完成
  - [x] 5.2 执行迁移 - Story 7-2 已完成

## Dev Notes

### 依赖关系

**前置条件**:
- Story 7.2（调用微信退款API）完成
- Story 7.3（扣减用户余额）完成

**注意**: 大部分实现已在 Story 7.3 的 `processRefundSuccess` 方法中完成，本 Story 主要是确保数据库 schema 正确和完善查询功能。

### 数据库 Schema

#### 1. 订单表字段扩展

在 `backend/ent/schema/recharge_order.go` 添加：

```go
package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
)

// RechargeOrder holds the schema definition for the RechargeOrder entity.
type RechargeOrder struct {
    ent.Schema
}

// Fields of the RechargeOrder.
func (RechargeOrder) Fields() []ent.Field {
    return []ent.Field{
        // ... 现有字段

        // 退款相关字段
        field.String("refund_no").
            Optional().
            Comment("退款单号"),

        field.String("refund_status").
            Optional().
            Comment("退款状态: pending/processing/success/failed"),

        field.Time("refunded_at").
            Optional().
            Comment("退款完成时间"),

        field.String("refund_reason").
            Optional().
            MaxLen(500).
            Comment("退款原因"),

        field.Int64("refund_admin_id").
            Optional().
            Comment("退款操作管理员ID"),
    }
}

// Indexes of the RechargeOrder.
func (RechargeOrder) Indexes() []ent.Index {
    return []ent.Index{
        // ... 现有索引

        // 退款单号索引
        index.Fields("refund_no"),
    }
}
```

#### 2. 余额日志表确认

确保 `backend/ent/schema/balance_log.go` 包含以下字段：

```go
package schema

import (
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
)

// BalanceLog holds the schema definition for the BalanceLog entity.
type BalanceLog struct {
    ent.Schema
}

// Fields of the BalanceLog.
func (BalanceLog) Fields() []ent.Field {
    return []ent.Field{
        field.Int64("id"),

        field.Int64("user_id").
            Comment("用户ID"),

        field.String("change_type").
            Comment("变动类型: recharge/consume/refund/adjust"),

        field.Float("amount").
            Comment("变动金额（正数增加，负数减少）"),

        field.Float("balance_before").
            Comment("变动前余额"),

        field.Float("balance_after").
            Comment("变动后余额"),

        field.String("related_order_no").
            Optional().
            Comment("关联订单号"),

        field.String("description").
            Optional().
            MaxLen(500).
            Comment("变动描述"),

        field.Int64("operator_id").
            Optional().
            Comment("操作人ID（system=0）"),

        field.String("operator_type").
            Optional().
            Comment("操作人类型: system/admin/user"),

        field.Time("created_at").
            Comment("创建时间"),
    }
}

// Indexes of the BalanceLog.
func (BalanceLog) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("user_id"),
        index.Fields("change_type"),
        index.Fields("related_order_no"),
        index.Fields("created_at"),
    }
}
```

### 数据库迁移

生成和应用迁移：

```bash
cd backend

# 生成迁移文件
go generate ./ent

# 应用迁移（如果使用 atlas 或其他迁移工具）
# 或者通过 ent 的 auto-migration
```

### 后端实现

#### 1. 订单状态更新（已在 Story 7.3 实现）

确保 `processRefundSuccess` 方法中包含以下更新：

```go
// 更新订单状态
_, err = tx.RechargeOrder.
    UpdateOneID(order.ID).
    SetStatus("refunded").
    SetRefundNo(params.RefundNo).
    SetRefundStatus("success").
    SetRefundReason(params.Reason).
    SetRefundAdminID(params.AdminID).
    SetRefundedAt(now).
    Save(ctx)
```

#### 2. 管理端退款查询

在 `backend/internal/handler/admin/recharge_handler.go` 添加：

```go
// ListRefundOrders 获取退款订单列表
// GET /api/v1/admin/recharge/orders?status=refunded
func (h *RechargeHandler) ListRefundOrders(c *gin.Context) {
    // 已在 ListOrders 中支持 status 筛选
}

// GetOrderRefundLogs 获取订单相关的余额日志
// GET /api/v1/admin/recharge/orders/:order_no/logs
func (h *RechargeHandler) GetOrderRefundLogs(c *gin.Context) {
    orderNo := c.Param("order_no")
    ctx := c.Request.Context()

    logs, err := h.rechargeService.GetBalanceLogsByOrderNo(ctx, orderNo)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "logs": logs,
    })
}
```

#### 3. Service 层查询方法

在 `backend/internal/service/recharge_service.go` 添加：

```go
// BalanceLogItem 余额日志项
type BalanceLogItem struct {
    ID            int64     `json:"id"`
    UserID        int64     `json:"user_id"`
    ChangeType    string    `json:"change_type"`
    Amount        float64   `json:"amount"`
    BalanceBefore float64   `json:"balance_before"`
    BalanceAfter  float64   `json:"balance_after"`
    RelatedOrderNo string   `json:"related_order_no"`
    Description   string    `json:"description"`
    OperatorID    int64     `json:"operator_id"`
    OperatorType  string    `json:"operator_type"`
    CreatedAt     time.Time `json:"created_at"`
}

// GetBalanceLogsByOrderNo 获取订单相关的余额日志
func (s *RechargeService) GetBalanceLogsByOrderNo(ctx context.Context, orderNo string) ([]*BalanceLogItem, error) {
    logs, err := s.db.BalanceLog.
        Query().
        Where(balancelog.RelatedOrderNoEQ(orderNo)).
        Order(ent.Desc(balancelog.FieldCreatedAt)).
        All(ctx)

    if err != nil {
        return nil, fmt.Errorf("query balance logs failed: %w", err)
    }

    items := make([]*BalanceLogItem, len(logs))
    for i, log := range logs {
        items[i] = &BalanceLogItem{
            ID:             log.ID,
            UserID:         log.UserID,
            ChangeType:     log.ChangeType,
            Amount:         log.Amount,
            BalanceBefore:  log.BalanceBefore,
            BalanceAfter:   log.BalanceAfter,
            RelatedOrderNo: log.RelatedOrderNo,
            Description:    log.Description,
            OperatorID:     log.OperatorID,
            OperatorType:   log.OperatorType,
            CreatedAt:      log.CreatedAt,
        }
    }

    return items, nil
}

// GetRefundStatistics 获取退款统计
func (s *RechargeService) GetRefundStatistics(ctx context.Context, startDate, endDate time.Time) (*RefundStatistics, error) {
    // 统计退款订单数和金额
    var result struct {
        Count       int     `json:"count"`
        TotalAmount float64 `json:"total_amount"`
    }

    err := s.db.RechargeOrder.
        Query().
        Where(
            rechargeorder.StatusEQ("refunded"),
            rechargeorder.RefundedAtGTE(startDate),
            rechargeorder.RefundedAtLT(endDate),
        ).
        Aggregate(
            ent.Count(),
            ent.Sum(rechargeorder.FieldAmount),
        ).
        Scan(ctx, &result)

    if err != nil {
        return nil, err
    }

    return &RefundStatistics{
        Count:       result.Count,
        TotalAmount: result.TotalAmount,
    }, nil
}

type RefundStatistics struct {
    Count       int     `json:"count"`
    TotalAmount float64 `json:"total_amount"`
}
```

### 前端实现

#### 1. 订单详情页显示退款信息（已在 Story 7.1 实现）

确保 `OrderDetailView.vue` 中显示退款信息：

```vue
<!-- 退款信息（如果已退款） -->
<div v-if="order.status === 'refunded'" class="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
  <h3 class="font-medium text-red-900 dark:text-red-200 mb-2">退款信息</h3>
  <div class="grid grid-cols-2 gap-4 text-sm">
    <InfoItem label="退款单号" :value="order.refund_no" />
    <InfoItem label="退款状态" :value="getRefundStatusText(order.refund_status)" />
    <InfoItem label="退款时间" :value="formatDate(order.refunded_at)" />
    <InfoItem label="退款原因" :value="order.refund_reason" />
    <InfoItem label="操作人ID" :value="order.refund_admin_id" />
  </div>
</div>
```

#### 2. 余额日志查看

在订单详情页添加余额日志标签页：

```vue
<!-- 余额变动日志 -->
<div class="mt-6">
  <h3 class="font-medium text-gray-900 dark:text-white mb-4">余额变动记录</h3>
  <table class="w-full text-sm">
    <thead class="bg-gray-50 dark:bg-gray-700">
      <tr>
        <th class="px-4 py-2 text-left">时间</th>
        <th class="px-4 py-2 text-left">类型</th>
        <th class="px-4 py-2 text-right">金额</th>
        <th class="px-4 py-2 text-right">余额变化</th>
        <th class="px-4 py-2 text-left">描述</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="log in balanceLogs" :key="log.id" class="border-b dark:border-gray-700">
        <td class="px-4 py-2">{{ formatDate(log.created_at) }}</td>
        <td class="px-4 py-2">
          <span :class="getChangeTypeBadgeClass(log.change_type)">
            {{ getChangeTypeText(log.change_type) }}
          </span>
        </td>
        <td class="px-4 py-2 text-right" :class="log.amount >= 0 ? 'text-green-600' : 'text-red-600'">
          {{ log.amount >= 0 ? '+' : '' }}{{ log.amount.toFixed(2) }}
        </td>
        <td class="px-4 py-2 text-right">
          {{ log.balance_before.toFixed(2) }} → {{ log.balance_after.toFixed(2) }}
        </td>
        <td class="px-4 py-2 text-gray-500">{{ log.description }}</td>
      </tr>
    </tbody>
  </table>
</div>
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/ent/schema/recharge_order.go` | 添加退款相关字段 |
| `backend/ent/schema/balance_log.go` | 确保日志表结构完整 |
| `backend/internal/service/recharge_service.go` | 添加日志查询方法 |
| `backend/internal/handler/admin/recharge_handler.go` | 添加日志查询接口 |
| `frontend/src/views/admin/recharge/OrderDetailView.vue` | 显示退款信息和日志 |

### 完整日志记录示例

一笔成功退款后的数据记录：

**订单表 (recharge_orders)**:
```json
{
  "order_no": "RECH20260124150000AbCd1234Ef",
  "status": "refunded",
  "amount": 100.00,
  "refund_no": "REFD20260124160000XyZw5678",
  "refund_status": "success",
  "refunded_at": "2026-01-24T16:00:00Z",
  "refund_reason": "用户申请退款",
  "refund_admin_id": 1
}
```

**余额日志表 (balance_logs)** - 完整流程会有两条记录：

```json
// 充值记录
{
  "user_id": 123,
  "change_type": "recharge",
  "amount": 100.00,
  "balance_before": 0.00,
  "balance_after": 100.00,
  "related_order_no": "RECH20260124150000AbCd1234Ef",
  "description": "充值到账",
  "operator_type": "system"
}

// 退款记录
{
  "user_id": 123,
  "change_type": "refund",
  "amount": -100.00,
  "balance_before": 100.00,
  "balance_after": 0.00,
  "related_order_no": "RECH20260124150000AbCd1234Ef",
  "description": "充值退款 - 用户申请退款",
  "operator_id": 1,
  "operator_type": "admin"
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-7.4] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/7-3-deduct-user-balance.md] - 扣减余额实现
- [Source: backend/ent/schema/] - 现有 Schema 参考

### 审计追溯

通过余额日志表可以完整追溯：

1. **充值流程**: change_type=recharge, operator_type=system
2. **退款流程**: change_type=refund, operator_type=admin, operator_id=管理员ID
3. **金额变化**: 通过 balance_before 和 balance_after 追溯

### 测试用例

```go
func TestRefundOrderComplete(t *testing.T) {
    // 1. 创建订单并支付
    // 2. 执行退款
    // 3. 验证订单状态为 refunded
    // 4. 验证退款字段正确填写
    // 5. 验证余额日志存在且正确
}

func TestGetBalanceLogsByOrderNo(t *testing.T) {
    // 1. 创建订单并支付
    // 2. 执行退款
    // 3. 查询日志，应返回充值和退款两条记录
}
```

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
