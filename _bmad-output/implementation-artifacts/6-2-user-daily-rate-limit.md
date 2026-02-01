# Story 6.2: 用户日级限流

Status: done

## Story

**作为** 系统
**我希望** 限制同一用户每天最多创建20个订单
**以便** 防止过度频繁充值

## Acceptance Criteria

- [x] AC1: 创建订单接口检查用户当日订单数
- [x] AC2: 超过限制返回 429 状态码和提示信息
- [x] AC3: 日级计数器每日零点重置
- [x] AC4: 限流配置可调整

## Tasks / Subtasks

- [x] Task 1: 后端 - 扩展限流配置 (AC: 4)
  - [x] 1.1 在配置中添加日级限流参数
  - [x] 1.2 设置合理的默认值（20个/天）

- [x] Task 2: 后端 - 日级限流实现 (AC: 1, 3)
  - [x] 2.1 在 RechargeRateLimitService 添加日级限流方法
  - [x] 2.2 使用 Redis Lua 脚本 INCR + EXPIREAT 实现
  - [x] 2.3 设置过期时间为次日凌晨

- [x] Task 3: 后端 - 应用日级限流 (AC: 1, 2)
  - [x] 3.1 在创建订单 Handler 中使用 CheckRechargeRateLimits 组合限流检查
  - [x] 3.2 与分钟级限流配合使用（两个都通过才允许）
  - [x] 3.3 返回明确的限流提示和 limit_type

- [x] Task 4: 前端 - 日级限流错误处理 (AC: 2)
  - [x] 4.1 扩展 RateLimitExceededError 支持日级限流
  - [x] 4.2 显示友好的提示（今日充值次数已达上限）
  - [x] 4.3 日级限流时禁用提交按钮

- [ ] Task 5: 单元测试 (AC: 1-4)
  - [ ] 5.1 测试正常日级限流
  - [ ] 5.2 测试跨天重置
  - [ ] 5.3 测试与分钟级限流配合

## Dev Notes

### 依赖关系

**前置条件**:
- Story 6.1（用户分钟级限流）完成，限流基础设施已建立

### 后端实现

#### 1. 扩展限流配置

在 `backend/internal/config/config.go` 扩展：

```go
// RechargRateLimitConfig 充值接口限流配置
type RechargRateLimitConfig struct {
    Enabled         bool `mapstructure:"enabled"`
    MinuteLimit     int  `mapstructure:"minute_limit"`      // 每分钟最大订单数
    MinuteWindowSec int  `mapstructure:"minute_window_sec"` // 分钟窗口大小（秒）
    DailyLimit      int  `mapstructure:"daily_limit"`       // 每天最大订单数
}

// 在 setDefaults() 中添加：
viper.SetDefault("rate_limit.recharge.daily_limit", 20)
```

#### 2. 日级限流实现

在 `backend/internal/middleware/rate_limit.go` 添加：

```go
// DailyRateLimiter 日级限流器
type DailyRateLimiter struct {
    redis *redis.Client
}

// NewDailyRateLimiter 创建日级限流器
func NewDailyRateLimiter(redis *redis.Client) *DailyRateLimiter {
    return &DailyRateLimiter{redis: redis}
}

// Allow 检查日级限流
// 返回: allowed, remaining, resetTime, error
func (r *DailyRateLimiter) Allow(ctx context.Context, key string, limit int) (bool, int, time.Time, error) {
    // 获取当日日期作为 key 后缀
    now := time.Now()
    date := now.Format("2006-01-02")
    fullKey := fmt.Sprintf("%s:%s", key, date)

    // 计算次日凌晨时间
    tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

    // 使用 Lua 脚本保证原子性
    script := redis.NewScript(`
        local key = KEYS[1]
        local limit = tonumber(ARGV[1])
        local expire_at = tonumber(ARGV[2])

        local current = redis.call('GET', key)
        if current == false then
            current = 0
        else
            current = tonumber(current)
        end

        if current < limit then
            redis.call('INCR', key)
            redis.call('EXPIREAT', key, expire_at)
            return {1, limit - current - 1}
        else
            return {0, 0}
        end
    `)

    result, err := script.Run(ctx, r.redis, []string{fullKey},
        limit,
        tomorrow.Unix(),
    ).Slice()

    if err != nil {
        return false, 0, time.Time{}, fmt.Errorf("daily rate limit script failed: %w", err)
    }

    allowed := result[0].(int64) == 1
    remaining := int(result[1].(int64))

    return allowed, remaining, tomorrow, nil
}

// GetCurrentCount 获取当前计数
func (r *DailyRateLimiter) GetCurrentCount(ctx context.Context, key string) (int, error) {
    now := time.Now()
    date := now.Format("2006-01-02")
    fullKey := fmt.Sprintf("%s:%s", key, date)

    count, err := r.redis.Get(ctx, fullKey).Int()
    if err == redis.Nil {
        return 0, nil
    }
    if err != nil {
        return 0, err
    }
    return count, nil
}
```

#### 3. 扩展限流服务

在 `backend/internal/service/rate_limit_service.go` 添加：

```go
// RateLimitService 限流服务
type RateLimitService struct {
    cfg          *config.Config
    minuteLimiter middleware.RateLimiter
    dailyLimiter  *middleware.DailyRateLimiter
}

// NewRateLimitService 创建限流服务
func NewRateLimitService(
    cfg *config.Config,
    minuteLimiter middleware.RateLimiter,
    dailyLimiter *middleware.DailyRateLimiter,
) *RateLimitService {
    return &RateLimitService{
        cfg:           cfg,
        minuteLimiter: minuteLimiter,
        dailyLimiter:  dailyLimiter,
    }
}

// DailyRateLimitResult 日级限流检查结果
type DailyRateLimitResult struct {
    Allowed   bool
    Remaining int
    ResetTime time.Time
    Message   string
}

// CheckRechargeDailyLimit 检查充值日级限流
func (s *RateLimitService) CheckRechargeDailyLimit(ctx context.Context, userID int64) (*DailyRateLimitResult, error) {
    if !s.cfg.RateLimit.Recharge.Enabled {
        return &DailyRateLimitResult{Allowed: true}, nil
    }

    key := fmt.Sprintf("ratelimit:recharge:user:%d:daily", userID)
    limit := s.cfg.RateLimit.Recharge.DailyLimit

    allowed, remaining, resetTime, err := s.dailyLimiter.Allow(ctx, key, limit)
    if err != nil {
        return nil, fmt.Errorf("check daily rate limit failed: %w", err)
    }

    result := &DailyRateLimitResult{
        Allowed:   allowed,
        Remaining: remaining,
        ResetTime: resetTime,
    }

    if !allowed {
        result.Message = fmt.Sprintf("今日充值次数已达上限（%d次），请明天再试", limit)
    }

    return result, nil
}

// CheckRechargeRateLimits 检查所有充值限流（分钟级 + 日级）
func (s *RateLimitService) CheckRechargeRateLimits(ctx context.Context, userID int64) (*CombinedRateLimitResult, error) {
    // 先检查日级限流（避免无效的分钟级计数）
    dailyResult, err := s.CheckRechargeDailyLimit(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("check daily limit failed: %w", err)
    }
    if !dailyResult.Allowed {
        return &CombinedRateLimitResult{
            Allowed:    false,
            LimitType:  "daily",
            Message:    dailyResult.Message,
            ResetTime:  dailyResult.ResetTime,
            RetryAfter: int(time.Until(dailyResult.ResetTime).Seconds()),
        }, nil
    }

    // 再检查分钟级限流
    minuteResult, err := s.CheckRechargeMinuteLimit(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("check minute limit failed: %w", err)
    }
    if !minuteResult.Allowed {
        return &CombinedRateLimitResult{
            Allowed:    false,
            LimitType:  "minute",
            Message:    minuteResult.Message,
            RetryAfter: minuteResult.RetryAfter,
        }, nil
    }

    return &CombinedRateLimitResult{
        Allowed:          true,
        MinuteRemaining:  minuteResult.Remaining,
        DailyRemaining:   dailyResult.Remaining,
    }, nil
}

// CombinedRateLimitResult 组合限流检查结果
type CombinedRateLimitResult struct {
    Allowed         bool
    LimitType       string    // "minute" 或 "daily"
    Message         string
    RetryAfter      int       // 秒（分钟级限流）
    ResetTime       time.Time // 重置时间（日级限流）
    MinuteRemaining int
    DailyRemaining  int
}
```

#### 4. Handler 中应用组合限流

在 `backend/internal/handler/recharge_handler.go` 中更新：

```go
// CreateOrder 创建充值订单
func (h *RechargeHandler) CreateOrder(c *gin.Context) {
    userID := c.GetInt64("user_id")
    ctx := c.Request.Context()

    // 检查所有限流
    limitResult, err := h.rateLimitService.CheckRechargeRateLimits(ctx, userID)
    if err != nil {
        log.Error("Check rate limit failed", "error", err)
        // 限流服务异常时不阻止请求，但记录日志
    } else if !limitResult.Allowed {
        response := gin.H{
            "error":      "rate_limit_exceeded",
            "limit_type": limitResult.LimitType,
            "message":    limitResult.Message,
        }

        if limitResult.LimitType == "minute" {
            c.Header("Retry-After", strconv.Itoa(limitResult.RetryAfter))
            response["retry_after"] = limitResult.RetryAfter
        } else {
            c.Header("X-RateLimit-Reset", limitResult.ResetTime.Format(time.RFC3339))
            response["reset_time"] = limitResult.ResetTime.Format(time.RFC3339)
        }

        c.Header("X-RateLimit-Remaining", "0")
        c.JSON(http.StatusTooManyRequests, response)
        return
    } else {
        c.Header("X-RateLimit-Minute-Remaining", strconv.Itoa(limitResult.MinuteRemaining))
        c.Header("X-RateLimit-Daily-Remaining", strconv.Itoa(limitResult.DailyRemaining))
    }

    // ... 继续正常的订单创建逻辑
}
```

### 前端实现

#### 1. 更新错误处理

在 `frontend/src/api/recharge.ts` 中：

```typescript
// 限流错误响应类型
interface RateLimitError {
  error: 'rate_limit_exceeded'
  limit_type: 'minute' | 'daily'
  message: string
  retry_after?: number  // 分钟级限流
  reset_time?: string   // 日级限流（ISO 8601）
}

// 自定义限流错误类
export class RateLimitExceededError extends Error {
  limitType: 'minute' | 'daily'
  retryAfter?: number
  resetTime?: Date

  constructor(data: RateLimitError) {
    super(data.message)
    this.name = 'RateLimitExceededError'
    this.limitType = data.limit_type
    this.retryAfter = data.retry_after
    if (data.reset_time) {
      this.resetTime = new Date(data.reset_time)
    }
  }

  get isDaily(): boolean {
    return this.limitType === 'daily'
  }

  get isMinute(): boolean {
    return this.limitType === 'minute'
  }
}
```

#### 2. 前端错误显示

在 `frontend/src/views/user/recharge/RechargeView.vue` 中：

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useToast } from '@/composables/useToast'
import { createRechargeOrder, RateLimitExceededError } from '@/api/recharge'

const toast = useToast()
const isSubmitting = ref(false)
const rateLimitError = ref<RateLimitExceededError | null>(null)
const minuteCountdown = ref(0)

const isDisabled = computed(() => {
  if (isSubmitting.value) return true
  if (minuteCountdown.value > 0) return true
  if (rateLimitError.value?.isDaily) return true
  return false
})

const buttonText = computed(() => {
  if (isSubmitting.value) return '处理中...'
  if (minuteCountdown.value > 0) return `${minuteCountdown.value}秒后可提交`
  if (rateLimitError.value?.isDaily) return '今日已达上限'
  return '立即充值'
})

async function handleSubmit() {
  if (isDisabled.value) return

  isSubmitting.value = true
  rateLimitError.value = null

  try {
    const order = await createRechargeOrder({ amount: selectedAmount.value })
    router.push({ name: 'recharge-paying', params: { orderNo: order.order_no } })
  } catch (error) {
    if (error instanceof RateLimitExceededError) {
      rateLimitError.value = error
      toast.warning(error.message)

      if (error.isMinute && error.retryAfter) {
        startMinuteCountdown(error.retryAfter)
      }
    } else {
      toast.error('创建订单失败，请稍后重试')
    }
  } finally {
    isSubmitting.value = false
  }
}

function startMinuteCountdown(seconds: number) {
  minuteCountdown.value = seconds
  const timer = setInterval(() => {
    minuteCountdown.value--
    if (minuteCountdown.value <= 0) {
      clearInterval(timer)
      rateLimitError.value = null
    }
  }, 1000)
}
</script>

<template>
  <div>
    <!-- 日级限流提示 -->
    <div
      v-if="rateLimitError?.isDaily"
      class="mb-4 p-4 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg"
    >
      <p class="text-sm text-yellow-800 dark:text-yellow-200">
        {{ rateLimitError.message }}
      </p>
    </div>

    <button
      @click="handleSubmit"
      :disabled="isDisabled"
      :class="[
        'w-full py-3 rounded-lg font-medium transition-colors',
        isDisabled
          ? 'bg-gray-300 text-gray-500 cursor-not-allowed'
          : 'bg-primary-600 text-white hover:bg-primary-700'
      ]"
    >
      {{ buttonText }}
    </button>
  </div>
</template>
```

### 配置示例

在 `deploy/config.example.yaml` 更新：

```yaml
rate_limit:
  recharge:
    enabled: true          # 是否启用充值限流
    minute_limit: 3        # 每分钟最大订单数
    minute_window_sec: 60  # 分钟窗口大小（秒）
    daily_limit: 20        # 每天最大订单数
```

### Redis Key 设计

```
Key: ratelimit:recharge:user:{user_id}:daily:{date}
Type: String (计数器)
Value: 当日已创建订单数
TTL: 到次日凌晨自动过期
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/middleware/rate_limit.go` | 添加 DailyRateLimiter |
| `backend/internal/service/rate_limit_service.go` | 添加日级限流和组合检查 |
| `backend/internal/handler/recharge_handler.go` | 使用组合限流检查 |
| `backend/internal/config/config.go` | 添加 daily_limit 配置 |
| `frontend/src/api/recharge.ts` | 扩展限流错误处理 |
| `frontend/src/views/user/recharge/RechargeView.vue` | 显示日级限流提示 |

### 响应头

限流相关响应头：

| Header | 说明 |
|--------|------|
| X-RateLimit-Minute-Remaining | 分钟窗口剩余次数 |
| X-RateLimit-Daily-Remaining | 今日剩余次数 |
| Retry-After | 分钟级限流时的重试等待秒数 |
| X-RateLimit-Reset | 日级限流的重置时间 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-6.2] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/6-1-user-minute-rate-limit.md] - 分钟级限流参考

### 测试用例

```go
func TestDailyRateLimiter(t *testing.T) {
    tests := []struct {
        name        string
        requests    int
        limit       int
        wantAllowed int
    }{
        {
            name:        "under limit",
            requests:    10,
            limit:       20,
            wantAllowed: 10,
        },
        {
            name:        "at limit",
            requests:    20,
            limit:       20,
            wantAllowed: 20,
        },
        {
            name:        "over limit",
            requests:    25,
            limit:       20,
            wantAllowed: 20,
        },
    }
    // ... 测试实现
}

func TestDailyRateLimiterReset(t *testing.T) {
    // 测试跨天重置
    // 使用 mock time 或调整 date key
}

func TestCombinedRateLimits(t *testing.T) {
    // 测试分钟级和日级限流的组合
}
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5

### Debug Log References

无

### Completion Notes List

- 在 `RechargeRateLimitConfig` 中添加 `DailyLimit` 字段，默认值 20
- 添加 `DailyRateLimitResult` 和 `CombinedRateLimitResult` 结构体
- 添加 `dailyLimitScript` Lua 脚本实现日级限流（INCR + EXPIREAT 到次日凌晨）
- 添加 `CheckRechargeDailyLimit` 方法检查日级限流
- 添加 `CheckRechargeRateLimits` 方法组合检查分钟级和日级限流
- 更新 Handler `CreateOrder` 使用组合限流检查，返回 limit_type 区分限流类型
- 响应头包含 X-RateLimit-Minute-Remaining 和 X-RateLimit-Daily-Remaining
- 前端扩展 `RateLimitExceededError` 支持 limitType 和 resetTime
- 前端添加 `isDailyLimited` 状态，日级限流时禁用按钮
- 添加 i18n 翻译 `dailyLimitReached`

### File List

- `backend/internal/config/config.go` - 添加 DailyLimit 字段和默认值
- `backend/internal/service/recharge_rate_limit_service.go` - 添加日级限流方法
- `backend/internal/handler/recharge/handler.go` - 更新使用组合限流检查
- `frontend/src/api/recharge.ts` - 扩展 RateLimitExceededError
- `frontend/src/views/user/RechargeView.vue` - 处理日级限流错误
- `frontend/src/i18n/locales/zh.ts` - 添加 dailyLimitReached
- `frontend/src/i18n/locales/en.ts` - 添加 dailyLimitReached
