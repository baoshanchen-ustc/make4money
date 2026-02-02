# Story 6.1: 用户分钟级限流

Status: done

## Story

**作为** 系统
**我希望** 限制同一用户1分钟内最多创建3个订单
**以便** 防止恶意刷单

## Acceptance Criteria

- [x] AC1: 创建订单接口检查用户分钟级订单数
- [x] AC2: 超过限制返回 429 状态码和提示信息
- [x] AC3: 使用 Redis 滑动窗口或令牌桶实现
- [x] AC4: 限流配置可调整

## Tasks / Subtasks

- [x] Task 1: 后端 - 创建限流服务 (AC: 1, 3)
  - [x] 1.1 创建 `backend/internal/service/recharge_rate_limit_service.go`
  - [x] 1.2 实现基于 Redis 的滑动窗口限流
  - [x] 1.3 支持不同的限流 key 和配置

- [x] Task 2: 后端 - 配置化限流参数 (AC: 4)
  - [x] 2.1 在配置文件中添加限流参数 (`rate_limit.recharge.*`)
  - [x] 2.2 支持启用/禁用、限制次数、窗口大小配置

- [x] Task 3: 后端 - 应用限流到创建订单接口 (AC: 1, 2)
  - [x] 3.1 在创建订单 Handler 中调用限流检查
  - [x] 3.2 超限时返回 429 状态码
  - [x] 3.3 返回友好的错误提示和重试时间

- [x] Task 4: 前端 - 限流错误处理 (AC: 2)
  - [x] 4.1 识别 429 状态码
  - [x] 4.2 显示友好的限流提示
  - [x] 4.3 显示剩余等待时间（倒计时）

## Dev Notes

### 依赖关系

**前置条件**:
- Redis 客户端已配置
- Story 2.5（充值订单创建）完成

### 后端实现

#### 1. 限流配置

在 `backend/internal/config/config.go` 添加：

```go
// RateLimitConfig 限流配置
type RateLimitConfig struct {
    Recharge RechargRateLimitConfig `mapstructure:"recharge"`
}

// RechargRateLimitConfig 充值接口限流配置
type RechargRateLimitConfig struct {
    Enabled         bool `mapstructure:"enabled"`
    MinuteLimit     int  `mapstructure:"minute_limit"`      // 每分钟最大订单数
    MinuteWindowSec int  `mapstructure:"minute_window_sec"` // 窗口大小（秒）
}

// 在 setDefaults() 中添加：
viper.SetDefault("rate_limit.recharge.enabled", true)
viper.SetDefault("rate_limit.recharge.minute_limit", 3)
viper.SetDefault("rate_limit.recharge.minute_window_sec", 60)
```

#### 2. 滑动窗口限流实现

创建 `backend/internal/middleware/rate_limit.go`：

```go
package middleware

import (
    "context"
    "fmt"
    "strconv"
    "time"

    "github.com/redis/go-redis/v9"
)

// RateLimiter 限流器接口
type RateLimiter interface {
    // Allow 检查是否允许请求
    // 返回: allowed, remaining, retryAfter(秒), error
    Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, int, error)
}

// SlidingWindowRateLimiter 滑动窗口限流器
type SlidingWindowRateLimiter struct {
    redis *redis.Client
}

// NewSlidingWindowRateLimiter 创建滑动窗口限流器
func NewSlidingWindowRateLimiter(redis *redis.Client) *SlidingWindowRateLimiter {
    return &SlidingWindowRateLimiter{redis: redis}
}

// Allow 检查是否允许请求（滑动窗口算法）
func (r *SlidingWindowRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, int, error) {
    now := time.Now()
    windowStart := now.Add(-window).UnixMilli()
    nowMilli := now.UnixMilli()

    // 使用 Lua 脚本保证原子性
    script := redis.NewScript(`
        local key = KEYS[1]
        local window_start = tonumber(ARGV[1])
        local now = tonumber(ARGV[2])
        local limit = tonumber(ARGV[3])
        local window_ms = tonumber(ARGV[4])

        -- 移除窗口外的请求
        redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

        -- 获取当前窗口内的请求数
        local count = redis.call('ZCARD', key)

        if count < limit then
            -- 添加当前请求
            redis.call('ZADD', key, now, now)
            -- 设置过期时间
            redis.call('PEXPIRE', key, window_ms)
            return {1, limit - count - 1, 0}
        else
            -- 获取最早的请求时间
            local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
            local retry_after = 0
            if #oldest > 0 then
                retry_after = math.ceil((tonumber(oldest[2]) + window_ms - now) / 1000)
                if retry_after < 0 then retry_after = 0 end
            end
            return {0, 0, retry_after}
        end
    `)

    result, err := script.Run(ctx, r.redis, []string{key},
        windowStart,
        nowMilli,
        limit,
        window.Milliseconds(),
    ).Slice()

    if err != nil {
        return false, 0, 0, fmt.Errorf("rate limit script failed: %w", err)
    }

    allowed := result[0].(int64) == 1
    remaining := int(result[1].(int64))
    retryAfter := int(result[2].(int64))

    return allowed, remaining, retryAfter, nil
}
```

#### 3. 限流服务

创建 `backend/internal/service/rate_limit_service.go`：

```go
package service

import (
    "context"
    "fmt"
    "time"

    "your-project/internal/config"
    "your-project/internal/middleware"
)

// RateLimitService 限流服务
type RateLimitService struct {
    cfg     *config.Config
    limiter middleware.RateLimiter
}

// NewRateLimitService 创建限流服务
func NewRateLimitService(cfg *config.Config, limiter middleware.RateLimiter) *RateLimitService {
    return &RateLimitService{
        cfg:     cfg,
        limiter: limiter,
    }
}

// RateLimitResult 限流检查结果
type RateLimitResult struct {
    Allowed    bool
    Remaining  int
    RetryAfter int // 秒
    Message    string
}

// CheckRechargeMinuteLimit 检查充值分钟级限流
func (s *RateLimitService) CheckRechargeMinuteLimit(ctx context.Context, userID int64) (*RateLimitResult, error) {
    if !s.cfg.RateLimit.Recharge.Enabled {
        return &RateLimitResult{Allowed: true}, nil
    }

    key := fmt.Sprintf("ratelimit:recharge:user:%d:minute", userID)
    limit := s.cfg.RateLimit.Recharge.MinuteLimit
    window := time.Duration(s.cfg.RateLimit.Recharge.MinuteWindowSec) * time.Second

    allowed, remaining, retryAfter, err := s.limiter.Allow(ctx, key, limit, window)
    if err != nil {
        return nil, fmt.Errorf("check rate limit failed: %w", err)
    }

    result := &RateLimitResult{
        Allowed:    allowed,
        Remaining:  remaining,
        RetryAfter: retryAfter,
    }

    if !allowed {
        result.Message = fmt.Sprintf("操作过于频繁，请在 %d 秒后重试", retryAfter)
    }

    return result, nil
}
```

#### 4. Handler 中应用限流

在 `backend/internal/handler/recharge_handler.go` 中：

```go
// CreateOrder 创建充值订单
func (h *RechargeHandler) CreateOrder(c *gin.Context) {
    userID := c.GetInt64("user_id")
    ctx := c.Request.Context()

    // 1. 检查分钟级限流
    limitResult, err := h.rateLimitService.CheckRechargeMinuteLimit(ctx, userID)
    if err != nil {
        log.Error("Check rate limit failed", "error", err)
        // 限流服务异常时不阻止请求，但记录日志
    } else if !limitResult.Allowed {
        c.Header("Retry-After", strconv.Itoa(limitResult.RetryAfter))
        c.Header("X-RateLimit-Remaining", "0")
        c.JSON(http.StatusTooManyRequests, gin.H{
            "error":       "rate_limit_exceeded",
            "message":     limitResult.Message,
            "retry_after": limitResult.RetryAfter,
        })
        return
    } else {
        c.Header("X-RateLimit-Remaining", strconv.Itoa(limitResult.Remaining))
    }

    // ... 继续正常的订单创建逻辑
}
```

#### 5. Wire 依赖注入

在 `backend/internal/middleware/wire.go` 添加：

```go
var ProviderSet = wire.NewSet(
    NewSlidingWindowRateLimiter,
)
```

在 `backend/internal/service/wire.go` 添加：

```go
var ProviderSet = wire.NewSet(
    // ... 现有 providers
    NewRateLimitService,
)
```

### 前端实现

#### 1. API 错误处理

在 `frontend/src/api/recharge.ts` 或全局拦截器中：

```typescript
// 限流错误响应类型
interface RateLimitError {
  error: 'rate_limit_exceeded'
  message: string
  retry_after: number
}

// 在创建订单时处理限流
export async function createRechargeOrder(params: CreateOrderParams): Promise<Order> {
  try {
    const { data } = await apiClient.post<Order>('/recharge/orders', params)
    return data
  } catch (error: any) {
    if (error.response?.status === 429) {
      const rateLimitError = error.response.data as RateLimitError
      throw new RateLimitExceededError(
        rateLimitError.message,
        rateLimitError.retry_after
      )
    }
    throw error
  }
}

// 自定义限流错误类
export class RateLimitExceededError extends Error {
  retryAfter: number

  constructor(message: string, retryAfter: number) {
    super(message)
    this.name = 'RateLimitExceededError'
    this.retryAfter = retryAfter
  }
}
```

#### 2. 前端错误显示

在 `frontend/src/views/user/recharge/RechargeView.vue` 中：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useToast } from '@/composables/useToast'
import { createRechargeOrder, RateLimitExceededError } from '@/api/recharge'

const toast = useToast()
const isSubmitting = ref(false)
const rateLimitCountdown = ref(0)

async function handleSubmit() {
  if (isSubmitting.value || rateLimitCountdown.value > 0) return

  isSubmitting.value = true
  try {
    const order = await createRechargeOrder({ amount: selectedAmount.value })
    router.push({ name: 'recharge-paying', params: { orderNo: order.order_no } })
  } catch (error) {
    if (error instanceof RateLimitExceededError) {
      toast.warning(error.message)
      startRateLimitCountdown(error.retryAfter)
    } else {
      toast.error('创建订单失败，请稍后重试')
    }
  } finally {
    isSubmitting.value = false
  }
}

function startRateLimitCountdown(seconds: number) {
  rateLimitCountdown.value = seconds
  const timer = setInterval(() => {
    rateLimitCountdown.value--
    if (rateLimitCountdown.value <= 0) {
      clearInterval(timer)
    }
  }, 1000)
}
</script>

<template>
  <button
    @click="handleSubmit"
    :disabled="isSubmitting || rateLimitCountdown > 0"
  >
    <span v-if="rateLimitCountdown > 0">
      {{ rateLimitCountdown }}秒后可再次提交
    </span>
    <span v-else-if="isSubmitting">
      处理中...
    </span>
    <span v-else>
      立即充值
    </span>
  </button>
</template>
```

### 配置示例

在 `deploy/config.example.yaml` 添加：

```yaml
rate_limit:
  recharge:
    enabled: true          # 是否启用充值限流
    minute_limit: 3        # 每分钟最大订单数
    minute_window_sec: 60  # 窗口大小（秒）
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/middleware/rate_limit.go` | 滑动窗口限流实现 |
| `backend/internal/service/rate_limit_service.go` | 限流服务封装 |
| `backend/internal/handler/recharge_handler.go` | 应用限流检查 |
| `backend/internal/config/config.go` | 添加限流配置 |
| `frontend/src/api/recharge.ts` | 处理 429 错误 |
| `frontend/src/views/user/recharge/RechargeView.vue` | 显示限流提示 |

### Redis Key 设计

```
Key: ratelimit:recharge:user:{user_id}:minute
Type: Sorted Set
Score: 请求时间戳（毫秒）
Value: 请求时间戳（毫秒）
TTL: 窗口大小（60秒）
```

### 响应头

限流相关响应头：

| Header | 说明 |
|--------|------|
| X-RateLimit-Remaining | 剩余可用次数 |
| Retry-After | 限流时，重试等待秒数 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-6.1] - 用户故事定义
- [Redis Rate Limiting](https://redis.io/commands/incr/) - Redis 限流实现参考
- [Sliding Window Algorithm](https://en.wikipedia.org/wiki/Sliding_window_protocol) - 滑动窗口算法

### 测试用例

```go
func TestSlidingWindowRateLimiter(t *testing.T) {
    tests := []struct {
        name        string
        requests    int
        limit       int
        window      time.Duration
        wantAllowed int
    }{
        {
            name:        "under limit",
            requests:    2,
            limit:       3,
            window:      time.Minute,
            wantAllowed: 2,
        },
        {
            name:        "at limit",
            requests:    3,
            limit:       3,
            window:      time.Minute,
            wantAllowed: 3,
        },
        {
            name:        "over limit",
            requests:    5,
            limit:       3,
            window:      time.Minute,
            wantAllowed: 3,
        },
    }
    // ... 测试实现
}

func TestRateLimitWindowExpiry(t *testing.T) {
    // 测试窗口过期后计数重置
}

func TestRateLimitConcurrency(t *testing.T) {
    // 测试并发请求的原子性
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
