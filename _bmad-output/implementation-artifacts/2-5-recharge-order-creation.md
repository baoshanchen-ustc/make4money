# Story 2.5: 充值订单创建

Status: done

## Story

**作为** 系统
**我希望** 创建充值订单并生成唯一订单号
**以便** 追踪支付流程和后续对账

## Acceptance Criteria

- [x] AC1: 订单号格式：RECH + 年月日时分秒 + 10位随机字符串（如：RECH20260124150000AbCd1234Ef）
- [x] AC2: 订单号全局唯一（数据库唯一索引）
- [x] AC3: 订单初始状态为 `pending`
- [x] AC4: 记录：user_id, amount, payment_method, payment_channel
- [x] AC5: 设置订单过期时间（created_at + expire_minutes）
- [x] AC6: 订单创建时间精确到毫秒

## Tasks / Subtasks

- [x] Task 1: 创建 `backend/ent/schema/recharge_order.go` Schema
- [x] Task 2: 实现订单号生成算法
- [x] Task 3: 实现订单创建 Service 方法
- [x] Task 4: 创建订单 Handler

## Dev Notes

### 数据库表

使用 `recharge_orders` 表存储

### 订单号生成

订单号生成使用时间戳+随机串

```go
func GenerateOrderNo() string {
    return fmt.Sprintf("RECH%s%s",
        time.Now().Format("20060102150405"),
        randomString(10))
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.5]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Completion Notes List

1. **Schema 已存在**：`backend/ent/schema/recharge_order.go` 已在之前 Story 中创建，包含所有必要字段
2. **配置扩展**：在 `WeChatPayConfig` 中添加 `OrderExpireMinutes` 字段，默认值 30 分钟
3. **订单号生成**：实现 `GenerateOrderNo()` 函数，格式 RECH+14位时间戳+10位加密随机字符串
4. **Service 层**：创建 `RechargeOrderService`，包含 `CreateOrder`、`GetOrder`、`GetOrderByID` 方法
5. **Repository 层**：创建 `rechargeOrderRepository`，实现 `Create`、`GetByID`、`GetByOrderNo`、`Update`、`ExistsByOrderNo`
6. **Handler 层**：扩展 `RechargeHandler`，添加 `CreateOrder` 端点
7. **路由注册**：在 `routes/user.go` 中注册 `/recharge/orders` POST 路由
8. **Wire 更新**：更新所有 wire.go 文件，重新生成依赖注入代码
9. **单元测试**：添加订单号生成算法测试，验证格式、唯一性、时间戳嵌入

### Code Review Notes

**Issues Found and Fixed:**
- HIGH: 添加支付渠道验证（handler 层验证 PaymentChannel 合法性）

### File List

**新增文件:**
- `backend/internal/service/recharge_order_service.go`
- `backend/internal/service/recharge_order_service_test.go`
- `backend/internal/repository/recharge_order_repo.go`

**修改文件:**
- `backend/internal/config/config.go` - 添加 OrderExpireMinutes 配置
- `backend/internal/handler/recharge/handler.go` - 添加 CreateOrder handler，支付渠道验证
- `backend/internal/handler/recharge/handler_test.go` - 更新测试以适配新签名
- `backend/internal/server/routes/user.go` - 注册充值订单路由
- `backend/internal/repository/wire.go` - 添加 RechargeOrderRepository
- `backend/internal/service/wire.go` - 添加 RechargeOrderService
- `backend/cmd/server/wire_gen.go` - 重新生成

## Change Log

- 2026-02-01: 完成 Story 2.5 实现，所有 AC 验证通过
- 2026-02-01: Code Review 完成，修复支付渠道验证问题
