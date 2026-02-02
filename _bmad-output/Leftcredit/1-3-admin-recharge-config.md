# Story 1.3: 管理端配置充值业务参数

Status: done

## Story

**作为** 系统管理员
**我希望** 在管理后台调整充值业务参数（金额选项、限额等）
**以便** 根据运营需求灵活调整充值配置

## Acceptance Criteria

- [x] AC1: 管理端可配置最小充值金额（默认1元）
- [x] AC2: 管理端可配置最大充值金额（默认1000元）
- [x] AC3: 管理端可配置默认充值金额选项（如：[10, 50, 100, 200, 500]）
- [x] AC4: 管理端可配置订单过期时间（分钟，默认120）
- [x] AC5: 输入验证：最小金额 <= 最大金额
- [x] AC6: 输入验证：金额选项在允许范围内
- [x] AC7: 保存时显示成功/失败提示

## Tasks / Subtasks

- [x] Task 1: 后端 - 定义充值配置常量和结构体 (AC: 1-4)
  - [x] 1.1 在 `backend/internal/service/domain_constants.go` 添加充值配置相关的 SettingKey 常量
  - [x] 1.2 在 `backend/internal/service/setting_service.go` 添加 `RechargeSettings` 结构体
  - [x] 1.3 添加默认值常量

- [x] Task 2: 后端 - 实现获取充值配置接口 (AC: 1-4)
  - [x] 2.1 在 `backend/internal/service/setting_service.go` 添加 `GetRechargeSettings()` 方法
  - [x] 2.2 在 `backend/internal/handler/admin/setting_handler.go` 添加 `GetRechargeSettings` Handler
  - [x] 2.3 在 `backend/internal/server/routes/admin.go` 注册 GET 路由

- [x] Task 3: 后端 - 实现更新充值配置接口 (AC: 1-7)
  - [x] 3.1 在 `backend/internal/handler/admin/setting_handler.go` 添加 `UpdateRechargeSettings` Handler
  - [x] 3.2 实现输入验证逻辑（最小金额 <= 最大金额，金额选项范围检查）
  - [x] 3.3 在 `backend/internal/service/setting_service.go` 添加 `UpdateRechargeSettings()` 方法
  - [x] 3.4 在 `backend/internal/server/routes/admin.go` 注册 PUT 路由

- [x] Task 4: 后端 - DTO 定义 (AC: 1-4)
  - [x] 4.1 在 `backend/internal/handler/dto/settings.go` 添加 `RechargeSettings` DTO

- [x] Task 5: 前端 - API 客户端 (AC: 1-4)
  - [x] 5.1 在 `frontend/src/api/admin/settings.ts` 添加 `RechargeSettings` 接口
  - [x] 5.2 添加 `getRechargeSettings()` 函数
  - [x] 5.3 添加 `updateRechargeSettings()` 函数

- [x] Task 6: 前端 - 充值配置表单组件 (AC: 1-7)
  - [x] 6.1 在 `frontend/src/views/admin/SettingsView.vue` 添加充值配置卡片
  - [x] 6.2 实现表单输入（最小金额、最大金额、金额选项、过期时间）
  - [x] 6.3 实现前端输入验证
  - [x] 6.4 实现保存功能和状态反馈

- [x] Task 7: 国际化 (AC: 1-7)
  - [x] 7.1 添加中文翻译 `frontend/src/i18n/locales/zh.ts`
  - [x] 7.2 添加英文翻译 `frontend/src/i18n/locales/en.ts`

- [ ] Task 8: 单元测试 (AC: 1-7)
  - [ ] 8.1 后端验证逻辑测试
  - [ ] 8.2 配置存储/读取测试

## Dev Notes

### 依赖关系

**前置条件**: Story 1.1 和 Story 1.2 应先完成（WeChatPayService 已创建）

本 Story 使用现有的 `settings` 表（key-value 模式）存储充值业务配置，无需新建表。

### 后端实现

#### 1. Setting Key 常量

在 `backend/internal/service/setting.go` 添加：

```go
const (
    // Recharge settings keys
    SettingKeyRechargeMinAmount        = "recharge.min_amount"
    SettingKeyRechargeMaxAmount        = "recharge.max_amount"
    SettingKeyRechargeDefaultAmounts   = "recharge.default_amounts"
    SettingKeyRechargeOrderExpireMinutes = "recharge.order_expire_minutes"
)

// Recharge defaults
const (
    DefaultRechargeMinAmount        = 1.0      // 最小充值金额（元）
    DefaultRechargeMaxAmount        = 1000.0   // 最大充值金额（元）
    DefaultRechargeOrderExpireMinutes = 120    // 订单过期时间（分钟）
)

var DefaultRechargeAmounts = []float64{10, 50, 100, 200, 500}
```

#### 2. RechargeSettings 结构体

在 `backend/internal/service/setting_service.go` 添加：

```go
// RechargeSettings 充值业务配置
type RechargeSettings struct {
    MinAmount          float64   `json:"min_amount"`           // 最小充值金额（元）
    MaxAmount          float64   `json:"max_amount"`           // 最大充值金额（元）
    DefaultAmounts     []float64 `json:"default_amounts"`      // 默认金额选项
    OrderExpireMinutes int       `json:"order_expire_minutes"` // 订单过期时间（分钟）
}

// GetRechargeSettings 获取充值业务配置
func (s *SettingService) GetRechargeSettings(ctx context.Context) (*RechargeSettings, error) {
    keys := []string{
        SettingKeyRechargeMinAmount,
        SettingKeyRechargeMaxAmount,
        SettingKeyRechargeDefaultAmounts,
        SettingKeyRechargeOrderExpireMinutes,
    }

    settings, err := s.settingRepo.GetMultiple(ctx, keys)
    if err != nil {
        return nil, fmt.Errorf("get recharge settings: %w", err)
    }

    result := &RechargeSettings{
        MinAmount:          DefaultRechargeMinAmount,
        MaxAmount:          DefaultRechargeMaxAmount,
        DefaultAmounts:     DefaultRechargeAmounts,
        OrderExpireMinutes: DefaultRechargeOrderExpireMinutes,
    }

    // 解析最小金额
    if raw, ok := settings[SettingKeyRechargeMinAmount]; ok && raw != "" {
        if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
            result.MinAmount = v
        }
    }

    // 解析最大金额
    if raw, ok := settings[SettingKeyRechargeMaxAmount]; ok && raw != "" {
        if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
            result.MaxAmount = v
        }
    }

    // 解析金额选项（JSON 数组）
    if raw, ok := settings[SettingKeyRechargeDefaultAmounts]; ok && raw != "" {
        var amounts []float64
        if err := json.Unmarshal([]byte(raw), &amounts); err == nil && len(amounts) > 0 {
            result.DefaultAmounts = amounts
        }
    }

    // 解析过期时间
    if raw, ok := settings[SettingKeyRechargeOrderExpireMinutes]; ok && raw != "" {
        if v, err := strconv.Atoi(raw); err == nil && v > 0 {
            result.OrderExpireMinutes = v
        }
    }

    return result, nil
}

// UpdateRechargeSettings 更新充值业务配置
func (s *SettingService) UpdateRechargeSettings(ctx context.Context, settings *RechargeSettings) error {
    // 验证逻辑
    if settings.MinAmount <= 0 {
        return fmt.Errorf("min_amount must be greater than 0")
    }
    if settings.MaxAmount <= 0 {
        return fmt.Errorf("max_amount must be greater than 0")
    }
    if settings.MinAmount > settings.MaxAmount {
        return fmt.Errorf("min_amount must be less than or equal to max_amount")
    }
    if settings.OrderExpireMinutes < 1 || settings.OrderExpireMinutes > 1440 {
        return fmt.Errorf("order_expire_minutes must be between 1 and 1440")
    }

    // 验证金额选项
    for _, amount := range settings.DefaultAmounts {
        if amount < settings.MinAmount || amount > settings.MaxAmount {
            return fmt.Errorf("default amount %.2f is out of allowed range [%.2f, %.2f]",
                amount, settings.MinAmount, settings.MaxAmount)
        }
    }

    // 序列化金额选项
    amountsJSON, err := json.Marshal(settings.DefaultAmounts)
    if err != nil {
        return fmt.Errorf("marshal default amounts: %w", err)
    }

    updates := map[string]string{
        SettingKeyRechargeMinAmount:          strconv.FormatFloat(settings.MinAmount, 'f', 2, 64),
        SettingKeyRechargeMaxAmount:          strconv.FormatFloat(settings.MaxAmount, 'f', 2, 64),
        SettingKeyRechargeDefaultAmounts:     string(amountsJSON),
        SettingKeyRechargeOrderExpireMinutes: strconv.Itoa(settings.OrderExpireMinutes),
    }

    return s.settingRepo.SetMultiple(ctx, updates)
}
```

#### 3. Handler 方法

在 `backend/internal/handler/admin/setting_handler.go` 添加：

```go
// GetRechargeSettings 获取充值业务配置
// GET /api/v1/admin/settings/recharge
func (h *SettingHandler) GetRechargeSettings(c *gin.Context) {
    settings, err := h.settingService.GetRechargeSettings(c.Request.Context())
    if err != nil {
        response.ErrorFrom(c, err)
        return
    }

    response.Success(c, dto.RechargeSettings{
        MinAmount:          settings.MinAmount,
        MaxAmount:          settings.MaxAmount,
        DefaultAmounts:     settings.DefaultAmounts,
        OrderExpireMinutes: settings.OrderExpireMinutes,
    })
}

// UpdateRechargeSettingsRequest 更新充值配置请求
type UpdateRechargeSettingsRequest struct {
    MinAmount          float64   `json:"min_amount" binding:"required,gt=0"`
    MaxAmount          float64   `json:"max_amount" binding:"required,gt=0"`
    DefaultAmounts     []float64 `json:"default_amounts" binding:"required,min=1"`
    OrderExpireMinutes int       `json:"order_expire_minutes" binding:"required,min=1,max=1440"`
}

// UpdateRechargeSettings 更新充值业务配置
// PUT /api/v1/admin/settings/recharge
func (h *SettingHandler) UpdateRechargeSettings(c *gin.Context) {
    var req UpdateRechargeSettingsRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request: "+err.Error())
        return
    }

    // 验证最小金额 <= 最大金额
    if req.MinAmount > req.MaxAmount {
        response.BadRequest(c, "Minimum amount must be less than or equal to maximum amount")
        return
    }

    // 验证金额选项在范围内
    for _, amount := range req.DefaultAmounts {
        if amount < req.MinAmount || amount > req.MaxAmount {
            response.BadRequest(c, fmt.Sprintf("Amount option %.2f is out of allowed range [%.2f, %.2f]",
                amount, req.MinAmount, req.MaxAmount))
            return
        }
    }

    settings := &service.RechargeSettings{
        MinAmount:          req.MinAmount,
        MaxAmount:          req.MaxAmount,
        DefaultAmounts:     req.DefaultAmounts,
        OrderExpireMinutes: req.OrderExpireMinutes,
    }

    if err := h.settingService.UpdateRechargeSettings(c.Request.Context(), settings); err != nil {
        response.ErrorFrom(c, err)
        return
    }

    // 重新获取设置返回
    updatedSettings, err := h.settingService.GetRechargeSettings(c.Request.Context())
    if err != nil {
        response.ErrorFrom(c, err)
        return
    }

    response.Success(c, dto.RechargeSettings{
        MinAmount:          updatedSettings.MinAmount,
        MaxAmount:          updatedSettings.MaxAmount,
        DefaultAmounts:     updatedSettings.DefaultAmounts,
        OrderExpireMinutes: updatedSettings.OrderExpireMinutes,
    })
}
```

#### 4. DTO 定义

在 `backend/internal/handler/dto/settings.go` 添加：

```go
// RechargeSettings 充值业务配置 DTO
type RechargeSettings struct {
    MinAmount          float64   `json:"min_amount"`           // 最小充值金额（元）
    MaxAmount          float64   `json:"max_amount"`           // 最大充值金额（元）
    DefaultAmounts     []float64 `json:"default_amounts"`      // 默认金额选项
    OrderExpireMinutes int       `json:"order_expire_minutes"` // 订单过期时间（分钟）
}
```

#### 5. 路由注册

在 `backend/internal/server/routes/admin.go` 添加：

```go
// 充值业务配置
adminSettings.GET("/recharge", h.Admin.Setting.GetRechargeSettings)
adminSettings.PUT("/recharge", h.Admin.Setting.UpdateRechargeSettings)
```

### 前端实现

#### 1. API 类型定义

在 `frontend/src/api/admin/settings.ts` 添加：

```typescript
// 充值业务配置
export interface RechargeSettings {
  min_amount: number
  max_amount: number
  default_amounts: number[]
  order_expire_minutes: number
}

// 获取充值业务配置
export async function getRechargeSettings(): Promise<RechargeSettings> {
  const { data } = await apiClient.get<RechargeSettings>('/admin/settings/recharge')
  return data
}

// 更新充值业务配置
export async function updateRechargeSettings(settings: RechargeSettings): Promise<RechargeSettings> {
  const { data } = await apiClient.put<RechargeSettings>('/admin/settings/recharge', settings)
  return data
}
```

#### 2. 国际化文本

**中文** (`frontend/src/locales/zh-CN.json`):
```json
{
  "admin": {
    "settings": {
      "recharge": {
        "title": "充值业务配置",
        "description": "配置用户充值的金额选项和订单规则",
        "minAmount": "最小充值金额",
        "maxAmount": "最大充值金额",
        "defaultAmounts": "默认金额选项",
        "addAmount": "添加金额",
        "orderExpireMinutes": "订单过期时间",
        "minutes": "分钟",
        "amountsHint": "用户在充值页面看到的快捷金额选项，金额必须在最小和最大范围内",
        "expireHint": "未支付订单的自动过期时间，范围 1-1440 分钟（24小时）",
        "saveSuccess": "充值配置已保存",
        "saveFailed": "保存充值配置失败",
        "errors": {
          "minGreaterThanMax": "最小金额不能大于最大金额",
          "amountOutOfRange": "金额选项必须在允许范围内"
        }
      }
    }
  }
}
```

**英文** (`frontend/src/locales/en.json`):
```json
{
  "admin": {
    "settings": {
      "recharge": {
        "title": "Recharge Settings",
        "description": "Configure recharge amount options and order rules",
        "minAmount": "Minimum Amount",
        "maxAmount": "Maximum Amount",
        "defaultAmounts": "Quick Amount Options",
        "addAmount": "Add Amount",
        "orderExpireMinutes": "Order Expiry Time",
        "minutes": "minutes",
        "amountsHint": "Quick amount options displayed on the recharge page. Amounts must be within the min/max range.",
        "expireHint": "Auto-expiry time for unpaid orders, range 1-1440 minutes (24 hours)",
        "saveSuccess": "Recharge settings saved",
        "saveFailed": "Failed to save recharge settings",
        "errors": {
          "minGreaterThanMax": "Minimum amount cannot be greater than maximum amount",
          "amountOutOfRange": "Amount options must be within the allowed range"
        }
      }
    }
  }
}
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/service/setting.go` | 添加 SettingKey 常量 |
| `backend/internal/service/setting_service.go` | 添加 RechargeSettings 结构体和方法 |
| `backend/internal/handler/admin/setting_handler.go` | 添加 Handler 方法 |
| `backend/internal/handler/dto/settings.go` | 添加 RechargeSettings DTO |
| `backend/internal/server/routes/admin.go` | 注册路由 |
| `frontend/src/api/admin/settings.ts` | 添加 API 函数和类型 |
| `frontend/src/views/admin/SettingsView.vue` | 添加充值配置卡片 |
| `frontend/src/locales/zh-CN.json` | 中文翻译 |
| `frontend/src/locales/en.json` | 英文翻译 |

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.3] - 用户故事定义
- [Source: backend/internal/service/setting_service.go] - 现有 SettingService 模式
- [Source: backend/internal/handler/admin/setting_handler.go] - 现有 Handler 模式
- [Source: frontend/src/api/admin/settings.ts] - 现有 API 模式
- [Source: frontend/src/views/admin/SettingsView.vue] - 现有设置页面模式

### 设计决策

1. **使用 settings 表存储**: 充值业务配置属于非敏感运营配置，使用现有 `settings` 表的 key-value 模式存储
2. **金额选项 JSON 序列化**: `default_amounts` 作为 JSON 数组字符串存储在 settings 表中
3. **前后端双重验证**: 前端提供即时反馈，后端作为安全防线
4. **默认值回退**: 数据库中无配置时使用代码中的默认值

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
