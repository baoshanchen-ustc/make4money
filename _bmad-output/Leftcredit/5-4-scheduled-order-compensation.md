# Story 5.4: 定时订单状态补偿任务

Status: done

## Story

**作为** 系统
**我希望** 定时扫描可能遗漏的订单并主动查询
**以便** 自动发现和补偿回调丢失的情况

## Acceptance Criteria

- [x] AC1: 定时任务每5分钟执行一次
- [x] AC2: 扫描 `status = pending` 且 `created_at < now() - 5分钟` 的订单
- [x] AC3: 对每个订单调用微信查询接口
- [x] AC4: 如果微信返回已支付则触发补偿到账
- [x] AC5: 每次最多处理50条
- [x] AC6: 记录补偿任务执行日志

## Tasks / Subtasks

- [x] Task 1: 后端 - 创建定时任务结构 (AC: 1)
  - [x] 1.1 创建 `backend/internal/service/order_compensation_scheduler.go`
  - [x] 1.2 使用 ticker 实现定时执行（间隔可配置，默认5分钟）
  - [x] 1.3 通过 Wire 自动启动

- [x] Task 2: 后端 - 订单扫描逻辑 (AC: 2, 5)
  - [x] 2.1 实现 GetPendingOrdersForCompensation 方法
  - [x] 2.2 添加创建时间过滤条件（>阈值分钟）
  - [x] 2.3 添加数量限制（批次大小可配置，默认50条）
  - [x] 2.4 按创建时间升序处理（先处理老订单）

- [x] Task 3: 后端 - 补偿处理逻辑 (AC: 3, 4)
  - [x] 3.1 遍历订单调用微信查询接口
  - [x] 3.2 微信返回 SUCCESS 时触发补偿到账
  - [x] 3.3 使用信号量控制并发数（可配置，默认5）
  - [x] 3.4 处理单个订单失败不影响其他订单

- [x] Task 4: 后端 - 日志记录 (AC: 6)
  - [x] 4.1 记录任务开始/结束日志
  - [x] 4.2 记录扫描到的订单数量
  - [x] 4.3 记录补偿成功/失败/跳过数量
  - [x] 4.4 记录单个订单处理结果

- [x] Task 5: 配置化 (AC: 1, 2, 5)
  - [x] 5.1 补偿间隔时间可配置 (compensation_interval_mins)
  - [x] 5.2 订单超时阈值可配置 (compensation_threshold_mins)
  - [x] 5.3 批次大小可配置 (compensation_batch_size)
  - [x] 5.4 并发数可配置 (compensation_concurrency)

## Dev Notes

### 依赖关系

**前置条件**:
- Story 5.2（查询微信支付订单状态）完成
- Story 5.3（补偿到账逻辑）完成

### 后端实现

#### 1. 定时任务配置

在 `backend/internal/config/config.go` 添加：

```go
// RechargeConfig 充值相关配置
type RechargeConfig struct {
    // ... 现有字段

    // 补偿任务配置
    CompensationEnabled       bool `mapstructure:"compensation_enabled"`        // 是否启用补偿任务
    CompensationIntervalMins  int  `mapstructure:"compensation_interval_mins"`  // 补偿任务执行间隔（分钟）
    CompensationThresholdMins int  `mapstructure:"compensation_threshold_mins"` // 订单超时阈值（分钟）
    CompensationBatchSize     int  `mapstructure:"compensation_batch_size"`     // 每批处理数量
    CompensationConcurrency   int  `mapstructure:"compensation_concurrency"`    // 并发查询数
}

// 在 setDefaults() 中添加：
viper.SetDefault("recharge.compensation_enabled", true)
viper.SetDefault("recharge.compensation_interval_mins", 5)
viper.SetDefault("recharge.compensation_threshold_mins", 5)
viper.SetDefault("recharge.compensation_batch_size", 50)
viper.SetDefault("recharge.compensation_concurrency", 5)
```

#### 2. 定时任务实现

创建 `backend/internal/cron/order_compensation_job.go`：

```go
package cron

import (
    "context"
    "sync"
    "time"

    "your-project/internal/config"
    "your-project/internal/log"
    "your-project/internal/service"
)

// OrderCompensationJob 订单补偿定时任务
type OrderCompensationJob struct {
    cfg             *config.Config
    rechargeService *service.RechargeService
    wechatPayService *service.WeChatPayService
    running         bool
    mu              sync.Mutex
    stopCh          chan struct{}
}

// NewOrderCompensationJob 创建订单补偿任务
func NewOrderCompensationJob(
    cfg *config.Config,
    rechargeService *service.RechargeService,
    wechatPayService *service.WeChatPayService,
) *OrderCompensationJob {
    return &OrderCompensationJob{
        cfg:             cfg,
        rechargeService: rechargeService,
        wechatPayService: wechatPayService,
        stopCh:          make(chan struct{}),
    }
}

// Start 启动定时任务
func (j *OrderCompensationJob) Start() {
    if !j.cfg.Recharge.CompensationEnabled {
        log.Info("Order compensation job is disabled")
        return
    }

    j.mu.Lock()
    if j.running {
        j.mu.Unlock()
        return
    }
    j.running = true
    j.mu.Unlock()

    interval := time.Duration(j.cfg.Recharge.CompensationIntervalMins) * time.Minute
    ticker := time.NewTicker(interval)

    log.Info("Order compensation job started",
        "interval_mins", j.cfg.Recharge.CompensationIntervalMins)

    go func() {
        // 启动后立即执行一次
        j.run()

        for {
            select {
            case <-ticker.C:
                j.run()
            case <-j.stopCh:
                ticker.Stop()
                log.Info("Order compensation job stopped")
                return
            }
        }
    }()
}

// Stop 停止定时任务
func (j *OrderCompensationJob) Stop() {
    j.mu.Lock()
    defer j.mu.Unlock()

    if j.running {
        close(j.stopCh)
        j.running = false
    }
}

// run 执行一次补偿任务
func (j *OrderCompensationJob) run() {
    ctx := context.Background()
    startTime := time.Now()

    log.Info("Order compensation job started")

    // 获取待补偿订单
    thresholdTime := time.Now().Add(-time.Duration(j.cfg.Recharge.CompensationThresholdMins) * time.Minute)
    orders, err := j.rechargeService.GetPendingOrdersForCompensation(ctx, thresholdTime, j.cfg.Recharge.CompensationBatchSize)
    if err != nil {
        log.Error("Failed to get pending orders for compensation", "error", err)
        return
    }

    if len(orders) == 0 {
        log.Info("No pending orders to compensate")
        return
    }

    log.Info("Found pending orders for compensation", "count", len(orders))

    // 使用信号量控制并发
    sem := make(chan struct{}, j.cfg.Recharge.CompensationConcurrency)
    var wg sync.WaitGroup
    var successCount, failCount int
    var mu sync.Mutex

    for _, order := range orders {
        wg.Add(1)
        sem <- struct{}{} // 获取信号量

        go func(orderNo string) {
            defer wg.Done()
            defer func() { <-sem }() // 释放信号量

            success := j.processOrder(ctx, orderNo)
            mu.Lock()
            if success {
                successCount++
            } else {
                failCount++
            }
            mu.Unlock()
        }(order.OrderNo)
    }

    wg.Wait()

    duration := time.Since(startTime)
    log.Info("Order compensation job completed",
        "total", len(orders),
        "success", successCount,
        "failed", failCount,
        "duration_ms", duration.Milliseconds())
}

// processOrder 处理单个订单
func (j *OrderCompensationJob) processOrder(ctx context.Context, orderNo string) bool {
    // 查询微信支付状态
    wechatResult, err := j.wechatPayService.QueryOrder(ctx, orderNo)
    if err != nil {
        log.Error("Failed to query wechat order",
            "order_no", orderNo,
            "error", err)
        return false
    }

    log.Info("Compensation: WeChat order status",
        "order_no", orderNo,
        "wechat_status", wechatResult.TradeState)

    // 如果微信显示已支付，触发补偿
    if wechatResult.TradeState == "SUCCESS" {
        err = j.rechargeService.ProcessPaymentSuccess(ctx, service.ProcessPaymentSuccessParams{
            OrderNo:       orderNo,
            TransactionID: wechatResult.TransactionID,
            Amount:        0,
            Source:        service.PaymentSourceCompensate,
        })
        if err != nil {
            log.Error("Failed to compensate order",
                "order_no", orderNo,
                "error", err)
            return false
        }

        log.Info("Order compensated successfully", "order_no", orderNo)
        return true
    }

    // 如果订单已关闭，更新本地状态
    if wechatResult.TradeState == "CLOSED" {
        err = j.rechargeService.MarkOrderExpired(ctx, orderNo)
        if err != nil {
            log.Error("Failed to mark order as expired",
                "order_no", orderNo,
                "error", err)
        }
    }

    return false
}
```

#### 3. Service 层方法

在 `backend/internal/service/recharge_service.go` 添加：

```go
// GetPendingOrdersForCompensation 获取待补偿的订单
func (s *RechargeService) GetPendingOrdersForCompensation(ctx context.Context, thresholdTime time.Time, limit int) ([]*ent.RechargeOrder, error) {
    orders, err := s.db.RechargeOrder.
        Query().
        Where(
            rechargeorder.StatusEQ("pending"),
            rechargeorder.CreatedAtLT(thresholdTime),
        ).
        Order(ent.Asc(rechargeorder.FieldCreatedAt)).
        Limit(limit).
        All(ctx)

    if err != nil {
        return nil, fmt.Errorf("query pending orders failed: %w", err)
    }

    return orders, nil
}

// MarkOrderExpired 标记订单为过期
func (s *RechargeService) MarkOrderExpired(ctx context.Context, orderNo string) error {
    _, err := s.db.RechargeOrder.
        Update().
        Where(
            rechargeorder.OrderNoEQ(orderNo),
            rechargeorder.StatusEQ("pending"),
        ).
        SetStatus("expired").
        Save(ctx)

    return err
}

// ProcessPaymentSuccess 导出为公开方法供外部调用
func (s *RechargeService) ProcessPaymentSuccess(ctx context.Context, params ProcessPaymentSuccessParams) error {
    return s.processPaymentSuccess(ctx, params)
}
```

#### 4. Wire 依赖注入

在 `backend/internal/cron/wire.go` 添加：

```go
package cron

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
    NewOrderCompensationJob,
    // ... 其他定时任务
)
```

#### 5. 应用启动注册

在 `backend/internal/server/server.go` 或 `main.go` 中：

```go
// 启动定时任务
orderCompensationJob.Start()

// 优雅关闭
defer orderCompensationJob.Stop()
```

### 配置示例

在 `deploy/config.example.yaml` 添加：

```yaml
recharge:
  # ... 其他配置

  # 订单补偿任务配置
  compensation_enabled: true        # 是否启用补偿任务
  compensation_interval_mins: 5     # 执行间隔（分钟）
  compensation_threshold_mins: 5    # 订单超时阈值（分钟），只处理创建时间超过此值的订单
  compensation_batch_size: 50       # 每次最多处理订单数
  compensation_concurrency: 5       # 并发查询数
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/cron/order_compensation_job.go` | 定时补偿任务实现 |
| `backend/internal/cron/wire.go` | 定时任务依赖注入 |
| `backend/internal/config/config.go` | 添加补偿任务配置 |
| `backend/internal/service/recharge_service.go` | 添加 GetPendingOrdersForCompensation 方法 |
| `deploy/config.example.yaml` | 配置示例 |

### 日志记录示例

```
INFO Order compensation job started
INFO Found pending orders for compensation count=3
INFO Compensation: WeChat order status order_no=RECH20260124150000AbCd1234Ef wechat_status=SUCCESS
INFO Order compensated successfully order_no=RECH20260124150000AbCd1234Ef
INFO Compensation: WeChat order status order_no=RECH20260124150001XyZw5678Gh wechat_status=NOTPAY
INFO Compensation: WeChat order status order_no=RECH20260124150002MnOp9012Ij wechat_status=CLOSED
INFO Order compensation job completed total=3 success=1 failed=0 duration_ms=1234
```

### 并发控制

1. **信号量限制**: 使用 channel 实现信号量，控制同时查询微信API的并发数
2. **独立错误处理**: 单个订单处理失败不影响其他订单
3. **统计汇总**: 记录成功/失败数量便于监控

### 监控指标

建议添加以下监控指标：

```go
// Prometheus metrics
var (
    compensationJobDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name: "recharge_compensation_job_duration_seconds",
        Help: "Duration of order compensation job",
    })
    compensationOrdersTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "recharge_compensation_orders_total",
        Help: "Total number of orders processed by compensation job",
    }, []string{"result"}) // result: success, failed, skipped
)
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-5.4] - 用户故事定义
- [Source: backend/internal/cron/] - 现有定时任务参考
- [Source: backend/internal/service/recharge_service.go] - 充值服务

### 测试用例

```go
func TestOrderCompensationJob(t *testing.T) {
    // 1. 测试无待补偿订单
    // 2. 测试有待补偿订单且微信返回 SUCCESS
    // 3. 测试有待补偿订单且微信返回 NOTPAY
    // 4. 测试有待补偿订单且微信返回 CLOSED
    // 5. 测试并发控制
    // 6. 测试单个订单失败不影响其他
}

func TestGetPendingOrdersForCompensation(t *testing.T) {
    // 1. 测试时间过滤条件
    // 2. 测试数量限制
    // 3. 测试排序（按创建时间升序）
}
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

无

### Completion Notes List

- 在 `WeChatPayConfig` 中添加补偿任务配置字段：CompensationEnabled、CompensationIntervalMins、CompensationThresholdMins、CompensationBatchSize、CompensationConcurrency
- 添加配置默认值：compensation_enabled=true、interval=5分钟、threshold=5分钟、batch=50、concurrency=5
- 创建 `OrderCompensationScheduler` 定时任务，使用 ticker 实现定时执行
- 支持启动时立即执行一次，然后按间隔周期执行
- 使用信号量控制并发查询微信API，避免请求过多
- 实现订单处理逻辑：SUCCESS→补偿到账、CLOSED→标记过期、PAYERROR→标记失败
- 在 `RechargeOrderRepository` 接口添加三个方法：GetPendingOrdersForCompensation、MarkOrderExpired、MarkOrderFailed
- 在 repository 实现层添加对应的数据库操作方法
- 通过 Wire 自动启动和清理，注册到 provideCleanup
- 更新 deploy/config.example.yaml 添加补偿配置示例

### File List

- `backend/internal/config/config.go` - 添加补偿任务配置字段和默认值
- `backend/internal/service/order_compensation_scheduler.go` - 新增定时补偿任务
- `backend/internal/service/recharge_order_service.go` - 更新 RechargeOrderRepository 接口
- `backend/internal/repository/recharge_order_repo.go` - 实现新增的仓储方法
- `backend/internal/service/wire.go` - 添加 ProvideOrderCompensationScheduler
- `backend/cmd/server/wire.go` - 添加 orderCompensationScheduler 到 cleanup
- `backend/cmd/server/wire_gen.go` - Wire 自动生成
- `deploy/config.example.yaml` - 添加补偿配置示例
