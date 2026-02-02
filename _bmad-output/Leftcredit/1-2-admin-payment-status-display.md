# Story 1.2: 管理端只读显示支付状态

Status: done

## Story

**作为** 系统管理员
**我希望** 在管理后台查看微信支付配置状态（脱敏显示）
**以便** 了解支付功能是否正常配置，无需访问服务器

## Acceptance Criteria

- [x] AC1: 管理端「系统设置」页面新增「支付配置」卡片
- [x] AC2: 显示微信支付启用状态（已启用/未启用）
- [x] AC3: 显示脱敏AppID（如：wx0b35f0****fb07e）
- [x] AC4: 显示回调地址（只读）
- [x] AC5: 所有敏感字段不可编辑
- [x] AC6: 配置未启用时显示「未配置」状态

## Tasks / Subtasks

- [x] Task 1: 后端 - 创建支付状态 API (AC: 1-6)
  - [x] 1.1 在 `backend/internal/handler/dto/settings.go` 添加 `WeChatPayStatus` DTO
  - [x] 1.2 在 `backend/internal/handler/admin/setting_handler.go` 添加 `GetWeChatPayStatus` 方法
  - [x] 1.3 在 `backend/internal/server/routes/admin.go` 注册路由

- [x] Task 2: 后端 - 实现脱敏逻辑 (AC: 3, 5)
  - [x] 2.1 实现 AppID 脱敏函数（保留前6位和后4位）
  - [x] 2.2 确保敏感字段（api_v3_key, cert_serial_no, private_key_path）不返回

- [x] Task 3: 前端 - 创建 API 客户端 (AC: 1)
  - [x] 3.1 在 `frontend/src/api/admin/settings.ts` 添加 `getWeChatPayStatus` 函数
  - [x] 3.2 定义 TypeScript 类型

- [x] Task 4: 前端 - 创建支付配置卡片组件 (AC: 1-6)
  - [x] 4.1 在 `frontend/src/views/admin/SettingsView.vue` 添加支付配置卡片
  - [x] 4.2 实现只读显示模式
  - [x] 4.3 实现脱敏显示和未配置状态

- [x] Task 5: 国际化 (AC: 1-6)
  - [x] 5.1 添加中文翻译 `frontend/src/i18n/locales/zh.ts`
  - [x] 5.2 添加英文翻译 `frontend/src/i18n/locales/en.ts`

## Dev Notes

### 依赖关系

**前置条件**: Story 1.1 必须先完成（WeChatPayConfig 结构体已定义）

本 Story 依赖 Story 1.1 中创建的：
- `config.WeChatPayConfig` 结构体
- `WeChatPayService` 服务

### 后端实现

#### 1. DTO 定义

在 `backend/internal/handler/dto/settings.go` 添加：

```go
// WeChatPayStatus 微信支付配置状态（只读，脱敏后返回）
type WeChatPayStatus struct {
    Enabled       bool   `json:"enabled"`         // 是否启用
    AppIDMasked   string `json:"app_id_masked"`   // 脱敏后的AppID
    MchIDMasked   string `json:"mch_id_masked"`   // 脱敏后的商户号
    NotifyURL     string `json:"notify_url"`      // 回调地址（可显示）
    Configured    bool   `json:"configured"`      // 是否已配置（所有必填字段都有值）
}
```

#### 2. 脱敏函数

在 `backend/internal/handler/admin/setting_handler.go` 或新建 `utils/mask.go`：

```go
// MaskString 脱敏字符串，保留前后指定位数
// 示例: MaskString("wx0b35f0a1b2fb07e", 6, 4) => "wx0b35****fb07e"
func MaskString(s string, keepPrefix, keepSuffix int) string {
    if s == "" {
        return ""
    }
    runes := []rune(s)
    length := len(runes)

    if length <= keepPrefix+keepSuffix {
        return s // 太短不脱敏
    }

    masked := make([]rune, length)
    for i := 0; i < length; i++ {
        if i < keepPrefix || i >= length-keepSuffix {
            masked[i] = runes[i]
        } else {
            masked[i] = '*'
        }
    }
    return string(masked)
}

// MaskAppID 脱敏微信AppID（保留前6后4）
func MaskAppID(appID string) string {
    return MaskString(appID, 6, 4)
}

// MaskMchID 脱敏商户号（保留前4后4）
func MaskMchID(mchID string) string {
    return MaskString(mchID, 4, 4)
}
```

#### 3. Handler 方法

在 `backend/internal/handler/admin/setting_handler.go` 添加：

```go
// GetWeChatPayStatus 获取微信支付配置状态（只读，脱敏）
// GET /api/v1/admin/payment/wechat/status
func (h *SettingHandler) GetWeChatPayStatus(c *gin.Context) {
    // 从 WeChatPayService 获取配置（需要依赖注入）
    cfg := h.wechatPayService.GetConfig()

    // 判断是否已配置（所有必填字段都有值）
    configured := cfg.Enabled &&
        cfg.AppID != "" &&
        cfg.MchID != "" &&
        cfg.APIv3Key != "" &&
        cfg.CertSerialNo != "" &&
        cfg.PrivateKeyPath != "" &&
        cfg.NotifyURL != ""

    status := &dto.WeChatPayStatus{
        Enabled:     cfg.Enabled,
        AppIDMasked: MaskAppID(cfg.AppID),
        MchIDMasked: MaskMchID(cfg.MchID),
        NotifyURL:   cfg.NotifyURL,
        Configured:  configured,
    }

    c.JSON(http.StatusOK, status)
}
```

#### 4. Handler 依赖注入更新

在 `setting_handler.go` 的结构体中添加：

```go
type SettingHandler struct {
    // ... 现有字段
    wechatPayService *service.WeChatPayService  // 新增
}

func NewSettingHandler(
    // ... 现有参数
    wechatPayService *service.WeChatPayService,  // 新增
) *SettingHandler {
    return &SettingHandler{
        // ... 现有赋值
        wechatPayService: wechatPayService,
    }
}
```

#### 5. 路由注册

在 `backend/internal/server/routes/admin.go` 的 `registerSettingsRoutes` 中添加：

```go
func registerSettingsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
    adminSettings := admin.Group("/settings")
    {
        // ... 现有路由

        // 微信支付状态（只读）
        adminSettings.GET("/payment/wechat/status", h.Admin.Setting.GetWeChatPayStatus)
    }
}
```

### 前端实现

#### 1. API 类型定义

在 `frontend/src/api/admin/settings.ts` 添加：

```typescript
// 微信支付状态（只读）
export interface WeChatPayStatus {
  enabled: boolean
  app_id_masked: string
  mch_id_masked: string
  notify_url: string
  configured: boolean
}

// 获取微信支付状态
export async function getWeChatPayStatus(): Promise<WeChatPayStatus> {
  const { data } = await apiClient.get<WeChatPayStatus>('/admin/settings/payment/wechat/status')
  return data
}
```

#### 2. 支付配置卡片

在 `frontend/src/views/admin/SettingsView.vue` 添加（参考现有卡片模式）：

```vue
<!-- 微信支付配置卡片（只读） -->
<div class="card">
  <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
      {{ t('admin.settings.wechatPay.title') }}
    </h2>
    <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.settings.wechatPay.description') }}
    </p>
  </div>

  <div class="space-y-4 p-6">
    <!-- 加载状态 -->
    <div v-if="wechatPayLoading" class="animate-pulse">
      <div class="h-4 bg-gray-200 rounded w-1/2 mb-4"></div>
      <div class="h-4 bg-gray-200 rounded w-3/4"></div>
    </div>

    <!-- 配置内容 -->
    <template v-else>
      <!-- 启用状态 -->
      <div class="flex items-center justify-between">
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.settings.wechatPay.status') }}
        </span>
        <span
          :class="[
            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
            wechatPayStatus?.enabled
              ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
              : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
          ]"
        >
          {{ wechatPayStatus?.enabled
            ? t('admin.settings.wechatPay.enabled')
            : t('admin.settings.wechatPay.disabled')
          }}
        </span>
      </div>

      <!-- 配置状态 -->
      <div class="flex items-center justify-between">
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.settings.wechatPay.configStatus') }}
        </span>
        <span
          :class="[
            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
            wechatPayStatus?.configured
              ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
              : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
          ]"
        >
          {{ wechatPayStatus?.configured
            ? t('admin.settings.wechatPay.configured')
            : t('admin.settings.wechatPay.notConfigured')
          }}
        </span>
      </div>

      <!-- AppID（脱敏） -->
      <div v-if="wechatPayStatus?.app_id_masked" class="flex items-center justify-between">
        <span class="text-sm text-gray-700 dark:text-gray-300">AppID</span>
        <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
          {{ wechatPayStatus.app_id_masked }}
        </code>
      </div>

      <!-- 商户号（脱敏） -->
      <div v-if="wechatPayStatus?.mch_id_masked" class="flex items-center justify-between">
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.settings.wechatPay.mchId') }}
        </span>
        <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
          {{ wechatPayStatus.mch_id_masked }}
        </code>
      </div>

      <!-- 回调地址 -->
      <div v-if="wechatPayStatus?.notify_url" class="flex items-center justify-between">
        <span class="text-sm text-gray-700 dark:text-gray-300">
          {{ t('admin.settings.wechatPay.notifyUrl') }}
        </span>
        <code class="text-sm font-mono text-gray-600 dark:text-gray-400 truncate max-w-xs">
          {{ wechatPayStatus.notify_url }}
        </code>
      </div>

      <!-- 未配置提示 -->
      <div v-if="!wechatPayStatus?.configured" class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
        <p class="text-sm text-yellow-800 dark:text-yellow-200">
          {{ t('admin.settings.wechatPay.configHint') }}
        </p>
      </div>
    </template>
  </div>
</div>
```

#### 3. 脚本逻辑

```typescript
// 在 script setup 中添加
import { adminAPI } from '@/api'
import type { WeChatPayStatus } from '@/api/admin/settings'

const wechatPayStatus = ref<WeChatPayStatus | null>(null)
const wechatPayLoading = ref(false)

async function loadWeChatPayStatus() {
  wechatPayLoading.value = true
  try {
    wechatPayStatus.value = await adminAPI.settings.getWeChatPayStatus()
  } catch (error: any) {
    console.error('Failed to load WeChat Pay status:', error)
    // 静默失败，不影响其他设置加载
  } finally {
    wechatPayLoading.value = false
  }
}

// 在 onMounted 中添加
onMounted(() => {
  // ... 现有加载
  loadWeChatPayStatus()
})
```

#### 4. 国际化文本

**中文** (`frontend/src/locales/zh-CN.json`):
```json
{
  "admin": {
    "settings": {
      "wechatPay": {
        "title": "微信支付配置",
        "description": "查看微信支付集成状态（敏感配置需在服务器 config.yaml 中修改）",
        "status": "支付状态",
        "enabled": "已启用",
        "disabled": "未启用",
        "configStatus": "配置状态",
        "configured": "已配置",
        "notConfigured": "未配置",
        "mchId": "商户号",
        "notifyUrl": "回调地址",
        "configHint": "微信支付配置需要在服务器的 config.yaml 文件中设置，包括 AppID、商户号、APIv3密钥等敏感信息。"
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
      "wechatPay": {
        "title": "WeChat Pay Configuration",
        "description": "View WeChat Pay integration status (sensitive config requires server config.yaml)",
        "status": "Payment Status",
        "enabled": "Enabled",
        "disabled": "Disabled",
        "configStatus": "Configuration",
        "configured": "Configured",
        "notConfigured": "Not Configured",
        "mchId": "Merchant ID",
        "notifyUrl": "Notify URL",
        "configHint": "WeChat Pay configuration must be set in the server's config.yaml file, including AppID, Merchant ID, APIv3 Key and other sensitive information."
      }
    }
  }
}
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/handler/dto/settings.go` | 添加 WeChatPayStatus DTO |
| `backend/internal/handler/admin/setting_handler.go` | 添加 GetWeChatPayStatus 方法 |
| `backend/internal/server/routes/admin.go` | 注册新路由 |
| `frontend/src/api/admin/settings.ts` | 添加 API 函数和类型 |
| `frontend/src/views/admin/SettingsView.vue` | 添加支付配置卡片 |
| `frontend/src/locales/zh-CN.json` | 中文翻译 |
| `frontend/src/locales/en.json` | 英文翻译 |

### 前一个 Story 的学习点

从 Story 1.1 的技术要点中获取的关键信息：
- `WeChatPayConfig` 结构体已在 `config.go` 中定义
- `WeChatPayService` 提供 `GetConfig()` 方法获取配置
- 配置字段：`Enabled`, `AppID`, `MchID`, `APIv3Key`, `CertSerialNo`, `PrivateKeyPath`, `NotifyURL`

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.2] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/1-1-load-wechat-pay-config.md] - 前置 Story 技术详情
- [Source: backend/internal/handler/dto/settings.go] - 现有 DTO 模式
- [Source: backend/internal/handler/admin/setting_handler.go] - 现有 Handler 模式
- [Source: frontend/src/views/admin/SettingsView.vue] - 现有设置页面模式

### 安全注意事项

1. **只读接口**: 此接口仅用于显示状态，不提供任何修改功能
2. **脱敏处理**: AppID 和商户号必须脱敏后返回
3. **敏感字段不返回**: APIv3Key、证书序列号、私钥路径等绝不返回
4. **管理员认证**: 接口需通过管理员认证中间件保护

## Dev Agent Record

### Agent Model Used

(待开发时填写)

### Debug Log References

(待开发时填写)

### Completion Notes List

(待开发时填写)

### File List

(待开发时填写)
