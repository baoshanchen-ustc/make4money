# Story 6.3: IP级限流与验证码

Status: done

## Story

**作为** 系统
**我希望** 同一IP短时间内创建大量订单时触发验证码验证
**以便** 防止机器人批量攻击

## Acceptance Criteria

- [x] AC1: 监控同一IP的订单创建频率
- [x] AC2: 超过阈值（如：10分钟内20个订单）触发验证码
- [x] AC3: 前端显示验证码输入框
- [x] AC4: 验证通过后允许继续创建订单
- [x] AC5: 验证码有效期5分钟

## Tasks / Subtasks

- [x] Task 1: 后端 - IP 订单计数 (AC: 1)
  - [x] 1.1 创建 IP 级限流检查方法
  - [x] 1.2 使用 Redis 记录 IP 订单创建计数
  - [x] 1.3 配置化阈值（10分钟20个）

- [x] Task 2: 后端 - 验证码生成与验证 (AC: 4, 5)
  - [x] 2.1 复用现有 Turnstile 验证码服务
  - [x] 2.2 通过 TurnstileService.VerifyToken 验证 token
  - [x] 2.3 Turnstile 自带有效期管理

- [x] Task 3: 后端 - 创建订单接口调整 (AC: 2)
  - [x] 3.1 检查 IP 是否需要验证码
  - [x] 3.2 需要验证码时返回 HTTP 428
  - [x] 3.3 验证请求中的验证码参数

- [x] Task 4: 前端 - 验证码显示 (AC: 3, 4)
  - [x] 4.1 识别需要验证码的响应 (428)
  - [x] 4.2 显示验证码组件
  - [x] 4.3 验证通过后重新提交订单

## Dev Notes

### 依赖关系

**前置条件**:
- Story 6.1, 6.2（用户限流）完成
- Turnstile 验证码服务已集成（或使用替代方案）

### 架构决策

**验证码方案选择**:
1. **方案A: 复用 Turnstile**（推荐）
   - 项目已集成 Cloudflare Turnstile
   - 无感验证，用户体验好
   - 参考 `backend/internal/service/turnstile_service.go`

2. **方案B: 图形验证码**
   - 自建简单图形验证码
   - 更高安全性但用户体验差

本文档以方案A（Turnstile）为例实现。

### 后端实现

#### 1. IP 限流配置

在 `backend/internal/config/config.go` 添加：

```go
// RechargRateLimitConfig 充值接口限流配置
type RechargRateLimitConfig struct {
    // ... 现有字段
    IPCaptchaThreshold  int `mapstructure:"ip_captcha_threshold"`   // IP 触发验证码的阈值
    IPCaptchaWindowMins int `mapstructure:"ip_captcha_window_mins"` // IP 统计窗口（分钟）
}

// 在 setDefaults() 中添加：
viper.SetDefault("rate_limit.recharge.ip_captcha_threshold", 20)
viper.SetDefault("rate_limit.recharge.ip_captcha_window_mins", 10)
```

#### 2. IP 限流实现

在 `backend/internal/middleware/rate_limit.go` 添加：

```go
// IPRateLimiter IP 级限流器
type IPRateLimiter struct {
    redis *redis.Client
}

// NewIPRateLimiter 创建 IP 限流器
func NewIPRateLimiter(redis *redis.Client) *IPRateLimiter {
    return &IPRateLimiter{redis: redis}
}

// CheckAndIncrement 检查 IP 计数并增加
// 返回: currentCount, needsCaptcha, error
func (r *IPRateLimiter) CheckAndIncrement(ctx context.Context, ip string, threshold int, window time.Duration) (int, bool, error) {
    key := fmt.Sprintf("ratelimit:recharge:ip:%s", ip)

    // Lua 脚本：获取当前计数并增加
    script := redis.NewScript(`
        local key = KEYS[1]
        local threshold = tonumber(ARGV[1])
        local window_sec = tonumber(ARGV[2])

        local count = redis.call('INCR', key)
        if count == 1 then
            redis.call('EXPIRE', key, window_sec)
        end

        return count
    `)

    count, err := script.Run(ctx, r.redis, []string{key},
        threshold,
        int64(window.Seconds()),
    ).Int()

    if err != nil {
        return 0, false, fmt.Errorf("ip rate limit script failed: %w", err)
    }

    needsCaptcha := count > threshold
    return count, needsCaptcha, nil
}

// GetCount 获取当前 IP 计数
func (r *IPRateLimiter) GetCount(ctx context.Context, ip string) (int, error) {
    key := fmt.Sprintf("ratelimit:recharge:ip:%s", ip)
    count, err := r.redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return 0, nil
    }
    return count, err
}
```

#### 3. 限流服务扩展

在 `backend/internal/service/rate_limit_service.go` 添加：

```go
// IPCaptchaResult IP 验证码检查结果
type IPCaptchaResult struct {
    NeedsCaptcha bool
    IPCount      int
    Message      string
}

// CheckIPCaptchaRequired 检查 IP 是否需要验证码
func (s *RateLimitService) CheckIPCaptchaRequired(ctx context.Context, ip string) (*IPCaptchaResult, error) {
    if !s.cfg.RateLimit.Recharge.Enabled {
        return &IPCaptchaResult{NeedsCaptcha: false}, nil
    }

    threshold := s.cfg.RateLimit.Recharge.IPCaptchaThreshold
    window := time.Duration(s.cfg.RateLimit.Recharge.IPCaptchaWindowMins) * time.Minute

    count, needsCaptcha, err := s.ipLimiter.CheckAndIncrement(ctx, ip, threshold, window)
    if err != nil {
        return nil, fmt.Errorf("check ip captcha failed: %w", err)
    }

    result := &IPCaptchaResult{
        NeedsCaptcha: needsCaptcha,
        IPCount:      count,
    }

    if needsCaptcha {
        result.Message = "检测到异常请求频率，请完成验证码验证"
    }

    return result, nil
}
```

#### 4. 创建订单请求 DTO

在 `backend/internal/handler/dto/recharge.go` 添加：

```go
// CreateRechargeOrderRequest 创建充值订单请求
type CreateRechargeOrderRequest struct {
    Amount         float64 `json:"amount" binding:"required,gt=0"`
    PaymentChannel string  `json:"payment_channel" binding:"required,oneof=native jsapi"`
    CaptchaToken   string  `json:"captcha_token"` // Turnstile 验证码 token（可选）
}
```

#### 5. Handler 中应用 IP 验证码

在 `backend/internal/handler/recharge_handler.go` 中更新：

```go
// CreateOrder 创建充值订单
func (h *RechargeHandler) CreateOrder(c *gin.Context) {
    var req dto.CreateRechargeOrderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
        return
    }

    userID := c.GetInt64("user_id")
    clientIP := c.ClientIP()
    ctx := c.Request.Context()

    // 1. 检查用户限流（分钟级 + 日级）
    limitResult, err := h.rateLimitService.CheckRechargeRateLimits(ctx, userID)
    if err != nil {
        log.Error("Check user rate limit failed", "error", err)
    } else if !limitResult.Allowed {
        // ... 返回用户限流错误
        return
    }

    // 2. 检查 IP 是否需要验证码
    ipResult, err := h.rateLimitService.CheckIPCaptchaRequired(ctx, clientIP)
    if err != nil {
        log.Error("Check IP captcha failed", "error", err)
    } else if ipResult.NeedsCaptcha {
        // 需要验证码但未提供
        if req.CaptchaToken == "" {
            c.JSON(http.StatusPreconditionRequired, gin.H{
                "error":           "captcha_required",
                "message":         ipResult.Message,
                "captcha_enabled": true,
            })
            return
        }

        // 验证 Turnstile token
        valid, err := h.turnstileService.Verify(ctx, req.CaptchaToken, clientIP)
        if err != nil {
            log.Error("Turnstile verify failed", "error", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "captcha verification failed"})
            return
        }
        if !valid {
            c.JSON(http.StatusBadRequest, gin.H{
                "error":   "captcha_invalid",
                "message": "验证码验证失败，请重试",
            })
            return
        }

        log.Info("Captcha verified for IP", "ip", clientIP)
    }

    // ... 继续正常的订单创建逻辑
}
```

### 前端实现

#### 1. Turnstile 组件（已存在）

项目已有 `frontend/src/components/TurnstileWidget.vue`，可直接复用。

#### 2. 创建订单时处理验证码

在 `frontend/src/views/user/recharge/RechargeView.vue` 中：

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useToast } from '@/composables/useToast'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { createRechargeOrder, RateLimitExceededError } from '@/api/recharge'

const toast = useToast()
const isSubmitting = ref(false)
const showCaptcha = ref(false)
const captchaToken = ref('')
const captchaKey = ref(0) // 用于重置 Turnstile

interface CaptchaRequiredError {
  error: 'captcha_required'
  message: string
  captcha_enabled: boolean
}

async function handleSubmit() {
  if (isSubmitting.value) return

  isSubmitting.value = true

  try {
    const order = await createRechargeOrder({
      amount: selectedAmount.value,
      payment_channel: 'native',
      captcha_token: captchaToken.value || undefined,
    })
    router.push({ name: 'recharge-paying', params: { orderNo: order.order_no } })
  } catch (error: any) {
    if (error.response?.status === 428 && error.response?.data?.error === 'captcha_required') {
      // 需要验证码
      const data = error.response.data as CaptchaRequiredError
      toast.warning(data.message)
      showCaptcha.value = true
      captchaToken.value = ''
      captchaKey.value++ // 重置验证码
    } else if (error.response?.data?.error === 'captcha_invalid') {
      toast.error('验证码验证失败，请重试')
      showCaptcha.value = true
      captchaToken.value = ''
      captchaKey.value++
    } else if (error instanceof RateLimitExceededError) {
      // ... 处理用户限流
    } else {
      toast.error('创建订单失败，请稍后重试')
    }
  } finally {
    isSubmitting.value = false
  }
}

function handleCaptchaSuccess(token: string) {
  captchaToken.value = token
  // 自动重新提交
  handleSubmit()
}

function handleCaptchaError() {
  toast.error('验证码加载失败，请刷新页面')
}
</script>

<template>
  <div class="space-y-6">
    <!-- 金额选择器 -->
    <!-- ... -->

    <!-- 验证码区域 -->
    <div v-if="showCaptcha" class="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
      <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
        检测到异常请求，请完成验证：
      </p>
      <TurnstileWidget
        :key="captchaKey"
        @success="handleCaptchaSuccess"
        @error="handleCaptchaError"
      />
    </div>

    <!-- 提交按钮 -->
    <button
      @click="handleSubmit"
      :disabled="isSubmitting || (showCaptcha && !captchaToken)"
      class="w-full py-3 rounded-lg font-medium bg-primary-600 text-white hover:bg-primary-700 disabled:bg-gray-300 disabled:cursor-not-allowed"
    >
      <span v-if="isSubmitting">处理中...</span>
      <span v-else-if="showCaptcha && !captchaToken">请完成验证</span>
      <span v-else>立即充值</span>
    </button>
  </div>
</template>
```

#### 3. API 客户端更新

在 `frontend/src/api/recharge.ts` 中：

```typescript
export interface CreateOrderParams {
  amount: number
  payment_channel: 'native' | 'jsapi'
  captcha_token?: string
}

export async function createRechargeOrder(params: CreateOrderParams): Promise<Order> {
  const { data } = await apiClient.post<Order>('/recharge/orders', params)
  return data
}
```

### 配置示例

在 `deploy/config.example.yaml` 更新：

```yaml
rate_limit:
  recharge:
    enabled: true             # 是否启用充值限流
    minute_limit: 3           # 每分钟最大订单数
    minute_window_sec: 60     # 分钟窗口大小（秒）
    daily_limit: 20           # 每天最大订单数
    ip_captcha_threshold: 20  # IP 触发验证码的阈值
    ip_captcha_window_mins: 10 # IP 统计窗口（分钟）
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/middleware/rate_limit.go` | 添加 IPRateLimiter |
| `backend/internal/service/rate_limit_service.go` | 添加 CheckIPCaptchaRequired |
| `backend/internal/handler/recharge_handler.go` | 验证码检查和验证 |
| `backend/internal/handler/dto/recharge.go` | 添加 captcha_token 字段 |
| `backend/internal/config/config.go` | 添加 IP 限流配置 |
| `frontend/src/views/user/recharge/RechargeView.vue` | 显示验证码 |
| `frontend/src/components/TurnstileWidget.vue` | Turnstile 组件（已存在） |

### Redis Key 设计

```
Key: ratelimit:recharge:ip:{ip}
Type: String (计数器)
Value: 窗口内订单创建数
TTL: 窗口大小（10分钟）
```

### HTTP 状态码

| 状态码 | 场景 |
|--------|------|
| 428 Precondition Required | 需要验证码但未提供 |
| 400 Bad Request | 验证码无效 |
| 429 Too Many Requests | 用户限流（分钟/日级） |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-6.3] - 用户故事定义
- [Source: backend/internal/service/turnstile_service.go] - Turnstile 服务
- [Source: frontend/src/components/TurnstileWidget.vue] - Turnstile 组件
- [Cloudflare Turnstile](https://developers.cloudflare.com/turnstile/)

### 安全考虑

1. **IP 伪造**: 使用 `c.ClientIP()` 获取真实 IP（需配置 trusted proxies）
2. **验证码绕过**: 服务端必须验证 token，不能仅依赖前端
3. **分布式攻击**: IP 限流对分布式攻击效果有限，结合用户限流更有效

### 测试用例

```go
func TestIPRateLimiter(t *testing.T) {
    // 1. 测试阈值以下不触发
    // 2. 测试超过阈值触发
    // 3. 测试窗口过期重置
}

func TestCaptchaFlow(t *testing.T) {
    // 1. 测试无验证码请求被拦截
    // 2. 测试有效验证码通过
    // 3. 测试无效验证码拒绝
}
```

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Completion Notes List

1. 添加了 IP 限流配置（IPCaptchaThreshold, IPCaptchaWindowMins）到 config.go
2. 在 RechargeRateLimitService 中实现 CheckIPCaptchaRequired 方法，使用 Lua 脚本原子计数
3. Handler 中添加 TurnstileService 依赖，检查 IP 验证码需求
4. 返回 HTTP 428 Precondition Required 表示需要验证码
5. 前端添加 CaptchaRequiredError 类型和处理逻辑
6. 在 RechargeView.vue 中集成 TurnstileWidget 组件
7. 添加中英文 i18n 翻译

### File List

**Backend:**
- `backend/internal/config/config.go` - 添加 IP 限流配置字段
- `backend/internal/service/recharge_rate_limit_service.go` - 添加 CheckIPCaptchaRequired 方法
- `backend/internal/handler/recharge/handler.go` - 添加 TurnstileService 依赖和 IP 验证码检查
- `backend/internal/handler/recharge/handler_test.go` - 更新测试构造函数参数

**Frontend:**
- `frontend/src/api/recharge.ts` - 添加 CaptchaRequiredError 和 captcha_token 字段
- `frontend/src/api/index.ts` - 导出 isCaptchaRequiredError
- `frontend/src/views/user/RechargeView.vue` - 集成 TurnstileWidget 和验证码处理
- `frontend/src/i18n/locales/zh.ts` - 添加验证码相关中文翻译
- `frontend/src/i18n/locales/en.ts` - 添加验证码相关英文翻译
