# Story 1.5: 配置优先级处理

Status: done

## Story

**作为** 系统开发者
**我希望** 数据库配置优先于文件配置
**以便** 支持运行时动态调整而保留默认值回退

## Acceptance Criteria

- [x] AC1: 系统优先读取数据库中的充值业务配置
- [x] AC2: 数据库中不存在该配置项时，使用 `config.yaml` 的默认值
- [x] AC3: 配置优先级：数据库 > config.yaml > 代码默认值
- [x] AC4: 配置读取逻辑封装在统一的配置服务中

## Tasks / Subtasks

- [x] Task 1: 后端 - 在 config.yaml 中添加充值配置默认值 (AC: 2)
  - [x] 1.1 在 `backend/internal/config/config.go` 添加 `RechargeConfig` 结构体
  - [x] 1.2 在 `Config` 结构体中添加 `Recharge RechargeConfig` 字段
  - [x] 1.3 在 `setDefaults()` 函数中设置默认值
  - [ ] 1.4 更新 `deploy/config.example.yaml` 添加示例配置 (skipped - 可选)

- [x] Task 2: 后端 - 实现分层配置读取 (AC: 1, 2, 3)
  - [x] 2.1 修改 `GetRechargeSettings()` 实现三层优先级读取
  - [x] 2.2 数据库配置为空时回退到 config.yaml
  - [x] 2.3 config.yaml 配置为空时回退到代码默认值

- [x] Task 3: 后端 - 封装统一的配置读取方法 (AC: 4)
  - [x] 3.1 实现 `GetRechargeConfigSources()` 方法（调试用）
  - [x] 3.2 文档化配置优先级规则（代码注释）

- [x] Task 4: 单元测试 (AC: 1-4)
  - [x] 4.1 测试数据库配置优先
  - [x] 4.2 测试回退到 config.yaml
  - [x] 4.3 测试回退到代码默认值

## Dev Notes

### 依赖关系

**前置条件**:
- Story 1.3（RechargeSettings 结构体已定义）
- Story 1.4（缓存机制已实现）

本 Story 在前两个 Story 的基础上，完善配置优先级处理逻辑。

### 配置优先级设计

```
┌─────────────────────────────────────────────────────────────┐
│                     配置读取优先级                            │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   1. 数据库 settings 表（最高优先级）                         │
│      ↓ 如果为空或未设置                                      │
│   2. config.yaml 文件配置                                    │
│      ↓ 如果为空或未设置                                      │
│   3. 代码中的默认常量（最低优先级）                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 设计原则

1. **运行时可调整**: 数据库配置允许管理员通过 Web 界面实时调整
2. **部署时可定制**: config.yaml 允许运维人员在部署时设置环境特定的默认值
3. **兜底安全**: 代码默认值确保系统即使配置缺失也能正常运行

### 后端实现

#### 1. config.yaml 配置结构

在 `backend/internal/config/config.go` 添加：

```go
// RechargeConfig 充值配置（config.yaml 中的默认值）
type RechargeConfig struct {
    MinAmount          float64   `mapstructure:"min_amount"`            // 最小充值金额（元）
    MaxAmount          float64   `mapstructure:"max_amount"`            // 最大充值金额（元）
    DefaultAmounts     []float64 `mapstructure:"default_amounts"`       // 默认金额选项
    OrderExpireMinutes int       `mapstructure:"order_expire_minutes"`  // 订单过期时间（分钟）
}

// 在 Config 结构体中添加
type Config struct {
    // ... 现有字段
    Recharge  RechargeConfig  `mapstructure:"recharge"`
    // ... 其他字段
}
```

#### 2. 默认值设置

在 `setDefaults()` 函数中添加：

```go
func setDefaults() {
    // ... 现有默认值

    // Recharge defaults
    viper.SetDefault("recharge.min_amount", 1.0)
    viper.SetDefault("recharge.max_amount", 1000.0)
    viper.SetDefault("recharge.default_amounts", []float64{10, 50, 100, 200, 500})
    viper.SetDefault("recharge.order_expire_minutes", 120)
}
```

#### 3. 配置示例文件

在 `deploy/config.example.yaml` 添加：

```yaml
# 充值业务配置（默认值，可通过管理后台覆盖）
recharge:
  min_amount: 1.0                          # 最小充值金额（元）
  max_amount: 1000.0                       # 最大充值金额（元）
  default_amounts: [10, 50, 100, 200, 500] # 快捷金额选项
  order_expire_minutes: 120                # 订单过期时间（分钟）
```

#### 4. 分层配置读取实现

修改 `backend/internal/service/setting_service.go`：

```go
// GetRechargeSettings 获取充值业务配置
// 配置优先级：数据库 > config.yaml > 代码默认值
func (s *SettingService) GetRechargeSettings(ctx context.Context) (*RechargeSettings, error) {
    // 尝试从缓存读取（如果已实现 Story 1.4）
    s.rechargeCacheMu.RLock()
    cache := s.rechargeCache
    s.rechargeCacheMu.RUnlock()

    if cache != nil && time.Since(cache.updatedAt) < s.rechargeCacheTTL {
        return copyRechargeSettings(cache.settings), nil
    }

    // 从数据库加载配置
    settings, err := s.loadRechargeSettingsWithFallback(ctx)
    if err != nil {
        return nil, err
    }

    // 更新缓存
    s.rechargeCacheMu.Lock()
    s.rechargeCache = &rechargeSettingsCache{
        settings:  settings,
        updatedAt: time.Now(),
    }
    s.rechargeCacheMu.Unlock()

    return copyRechargeSettings(settings), nil
}

// loadRechargeSettingsWithFallback 从数据库加载配置，支持多层回退
func (s *SettingService) loadRechargeSettingsWithFallback(ctx context.Context) (*RechargeSettings, error) {
    keys := []string{
        SettingKeyRechargeMinAmount,
        SettingKeyRechargeMaxAmount,
        SettingKeyRechargeDefaultAmounts,
        SettingKeyRechargeOrderExpireMinutes,
    }

    dbSettings, err := s.settingRepo.GetMultiple(ctx, keys)
    if err != nil {
        return nil, fmt.Errorf("get recharge settings from db: %w", err)
    }

    result := &RechargeSettings{}

    // 1. 最小充值金额：DB > config.yaml > 代码默认值
    result.MinAmount = s.getFloatWithFallback(
        dbSettings[SettingKeyRechargeMinAmount],
        s.cfg.Recharge.MinAmount,
        DefaultRechargeMinAmount,
    )

    // 2. 最大充值金额：DB > config.yaml > 代码默认值
    result.MaxAmount = s.getFloatWithFallback(
        dbSettings[SettingKeyRechargeMaxAmount],
        s.cfg.Recharge.MaxAmount,
        DefaultRechargeMaxAmount,
    )

    // 3. 默认金额选项：DB > config.yaml > 代码默认值
    result.DefaultAmounts = s.getAmountsWithFallback(
        dbSettings[SettingKeyRechargeDefaultAmounts],
        s.cfg.Recharge.DefaultAmounts,
        DefaultRechargeAmounts,
    )

    // 4. 订单过期时间：DB > config.yaml > 代码默认值
    result.OrderExpireMinutes = s.getIntWithFallback(
        dbSettings[SettingKeyRechargeOrderExpireMinutes],
        s.cfg.Recharge.OrderExpireMinutes,
        DefaultRechargeOrderExpireMinutes,
    )

    return result, nil
}

// getFloatWithFallback 获取浮点数配置，支持三层回退
func (s *SettingService) getFloatWithFallback(dbValue string, configValue float64, defaultValue float64) float64 {
    // 尝试从数据库读取
    if dbValue != "" {
        if v, err := strconv.ParseFloat(dbValue, 64); err == nil && v > 0 {
            return v
        }
    }

    // 尝试从 config.yaml 读取
    if configValue > 0 {
        return configValue
    }

    // 使用代码默认值
    return defaultValue
}

// getIntWithFallback 获取整数配置，支持三层回退
func (s *SettingService) getIntWithFallback(dbValue string, configValue int, defaultValue int) int {
    // 尝试从数据库读取
    if dbValue != "" {
        if v, err := strconv.Atoi(dbValue); err == nil && v > 0 {
            return v
        }
    }

    // 尝试从 config.yaml 读取
    if configValue > 0 {
        return configValue
    }

    // 使用代码默认值
    return defaultValue
}

// getAmountsWithFallback 获取金额数组配置，支持三层回退
func (s *SettingService) getAmountsWithFallback(dbValue string, configValue []float64, defaultValue []float64) []float64 {
    // 尝试从数据库读取
    if dbValue != "" {
        var amounts []float64
        if err := json.Unmarshal([]byte(dbValue), &amounts); err == nil && len(amounts) > 0 {
            return amounts
        }
    }

    // 尝试从 config.yaml 读取
    if len(configValue) > 0 {
        return configValue
    }

    // 使用代码默认值
    return defaultValue
}

// copyRechargeSettings 返回配置的副本，避免外部修改
func copyRechargeSettings(settings *RechargeSettings) *RechargeSettings {
    return &RechargeSettings{
        MinAmount:          settings.MinAmount,
        MaxAmount:          settings.MaxAmount,
        DefaultAmounts:     append([]float64{}, settings.DefaultAmounts...),
        OrderExpireMinutes: settings.OrderExpireMinutes,
    }
}
```

#### 5. 辅助方法：获取有效配置来源（用于调试）

```go
// GetRechargeConfigSources 获取各配置项的实际来源（用于调试）
func (s *SettingService) GetRechargeConfigSources(ctx context.Context) (map[string]string, error) {
    keys := []string{
        SettingKeyRechargeMinAmount,
        SettingKeyRechargeMaxAmount,
        SettingKeyRechargeDefaultAmounts,
        SettingKeyRechargeOrderExpireMinutes,
    }

    dbSettings, err := s.settingRepo.GetMultiple(ctx, keys)
    if err != nil {
        return nil, err
    }

    sources := make(map[string]string)

    // 判断各配置项的来源
    if dbSettings[SettingKeyRechargeMinAmount] != "" {
        sources["min_amount"] = "database"
    } else if s.cfg.Recharge.MinAmount > 0 {
        sources["min_amount"] = "config.yaml"
    } else {
        sources["min_amount"] = "default"
    }

    if dbSettings[SettingKeyRechargeMaxAmount] != "" {
        sources["max_amount"] = "database"
    } else if s.cfg.Recharge.MaxAmount > 0 {
        sources["max_amount"] = "config.yaml"
    } else {
        sources["max_amount"] = "default"
    }

    if dbSettings[SettingKeyRechargeDefaultAmounts] != "" {
        sources["default_amounts"] = "database"
    } else if len(s.cfg.Recharge.DefaultAmounts) > 0 {
        sources["default_amounts"] = "config.yaml"
    } else {
        sources["default_amounts"] = "default"
    }

    if dbSettings[SettingKeyRechargeOrderExpireMinutes] != "" {
        sources["order_expire_minutes"] = "database"
    } else if s.cfg.Recharge.OrderExpireMinutes > 0 {
        sources["order_expire_minutes"] = "config.yaml"
    } else {
        sources["order_expire_minutes"] = "default"
    }

    return sources, nil
}
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/config/config.go` | 添加 RechargeConfig 结构体 |
| `backend/internal/service/setting.go` | 确保默认值常量存在 |
| `backend/internal/service/setting_service.go` | 实现分层配置读取 |
| `deploy/config.example.yaml` | 添加配置示例 |

### 测试场景

```go
func TestRechargeConfigPriority(t *testing.T) {
    // 场景1: 数据库有值，应使用数据库值
    t.Run("DatabasePriority", func(t *testing.T) {
        // 设置数据库中 min_amount = 5.0
        // config.yaml 中 min_amount = 2.0
        // 代码默认值 = 1.0
        // 期望返回 5.0
    })

    // 场景2: 数据库无值，应回退到 config.yaml
    t.Run("FallbackToConfig", func(t *testing.T) {
        // 数据库中 min_amount 为空
        // config.yaml 中 min_amount = 2.0
        // 代码默认值 = 1.0
        // 期望返回 2.0
    })

    // 场景3: 数据库和 config.yaml 都无值，应使用代码默认值
    t.Run("FallbackToDefault", func(t *testing.T) {
        // 数据库中 min_amount 为空
        // config.yaml 中 min_amount = 0 (未设置)
        // 代码默认值 = 1.0
        // 期望返回 1.0
    })

    // 场景4: 部分配置来自不同来源
    t.Run("MixedSources", func(t *testing.T) {
        // min_amount 来自数据库
        // max_amount 来自 config.yaml
        // default_amounts 来自代码默认值
        // 验证各字段正确合并
    })
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.5] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/1-3-admin-recharge-config.md] - Story 1.3 定义
- [Source: _bmad-output/implementation-artifacts/1-4-config-realtime-effect.md] - Story 1.4 定义
- [Source: backend/internal/config/config.go] - 现有配置结构参考
- [Source: backend/internal/service/setting_service.go] - 现有配置读取模式参考（如 LinuxDoConnectOAuthConfig）

### 设计决策

1. **三层优先级**: 提供最大灵活性，满足不同场景需求
2. **每个字段独立回退**: 支持混合配置来源，部分字段来自数据库，部分来自文件
3. **正值校验**: 使用 `> 0` 判断配置是否有效，避免零值被误认为有效配置
4. **配置来源追溯**: 提供 `GetRechargeConfigSources()` 方法用于调试和运维

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A

### Completion Notes List

1. **RechargeConfig 结构体**：在 `config.go` 中添加，包含 MinAmount、MaxAmount、DefaultAmounts、OrderExpireMinutes 四个字段
2. **setDefaults 默认值**：添加了 recharge.* 的默认值配置
3. **三层回退机制**：
   - `loadRechargeSettingsFromDB()` 重构为支持三层优先级
   - 新增 `getFloatWithFallback()`、`getIntWithFallback()`、`getAmountsWithFallback()` 辅助方法
4. **配置来源追溯**：新增 `GetRechargeConfigSources()` 方法，返回每个配置项的实际来源
5. **单元测试**：新增 5 个测试用例覆盖各种配置优先级场景

### File List

- backend/internal/config/config.go (修改)
- backend/internal/service/setting_service.go (修改)
- backend/internal/service/setting_service_test.go (修改)
