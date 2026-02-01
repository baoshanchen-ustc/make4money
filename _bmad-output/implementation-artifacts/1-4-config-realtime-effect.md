# Story 1.4: 配置实时生效机制

Status: done

## Story

**作为** 系统管理员
**我希望** 修改充值配置后立即生效，无需重启服务
**以便** 快速响应运营需求变化

## Acceptance Criteria

- [x] AC1: 配置保存到数据库 `settings` 表
- [x] AC2: 配置变更后无需重启服务即可生效
- [x] AC3: 使用内存缓存 + 定期刷新机制（1分钟）
- [x] AC4: 配置更新后主动刷新缓存（实时生效）

## Tasks / Subtasks

- [x] Task 1: 后端 - 设计缓存结构 (AC: 1-3)
  - [x] 1.1 在 `backend/internal/service/setting_service.go` 添加内存缓存字段
  - [x] 1.2 添加 sync.RWMutex 保护缓存读写
  - [x] 1.3 定义缓存过期时间常量（60秒）

- [x] Task 2: 后端 - 实现缓存刷新机制 (AC: 2, 3)
  - [x] 2.1 实现 `refreshRechargeSettingsCache()` 方法
  - [x] 2.2 实现定时刷新 goroutine（使用 time.Ticker）
  - [x] 2.3 在 `NewSettingService()` 中启动后台刷新任务

- [x] Task 3: 后端 - 实现缓存读取逻辑 (AC: 2, 3)
  - [x] 3.1 修改 `GetRechargeSettings()` 优先从缓存读取
  - [x] 3.2 缓存未命中时从数据库加载并更新缓存
  - [x] 3.3 实现缓存过期判断逻辑

- [x] Task 4: 后端 - 实现主动失效机制 (AC: 4)
  - [x] 4.1 在 `UpdateRechargeSettings()` 中更新后主动刷新缓存
  - [x] 4.2 确保写入数据库成功后再更新缓存

- [x] Task 5: 后端 - 优雅关闭 (AC: 2)
  - [x] 5.1 实现 `Stop()` 方法停止后台刷新任务
  - [x] 5.2 在应用退出时调用清理方法

- [x] Task 6: 单元测试 (AC: 1-4)
  - [x] 6.1 测试缓存命中场景
  - [x] 6.2 测试缓存过期后自动刷新
  - [x] 6.3 测试配置更新后缓存失效

## Dev Notes

### 依赖关系

**前置条件**: Story 1.3 必须先完成（RechargeSettings 结构体和方法已定义）

本 Story 在 Story 1.3 的基础上，为充值配置增加内存缓存层，提升读取性能并实现配置实时生效。

### 设计方案

#### 方案选择：内存缓存 + 定期刷新 + 主动失效

选择此方案的原因：
1. **简单可靠**: 不依赖外部缓存系统（如 Redis），减少复杂度
2. **适合场景**: 充值配置读多写少，缓存命中率高
3. **实时性保证**: 结合主动失效，配置更新后立即生效
4. **资源占用低**: 单一配置对象，内存占用可忽略不计

#### 缓存策略

- **缓存粒度**: 整个 `RechargeSettings` 结构体作为一个缓存对象
- **缓存有效期**: 60秒（作为兜底，正常情况下由主动失效保证实时性）
- **刷新方式**:
  - 定期刷新：每60秒自动从数据库同步
  - 主动失效：配置更新后立即刷新缓存

### 后端实现

#### 1. 缓存结构体定义

在 `backend/internal/service/setting_service.go` 添加：

```go
import (
    "sync"
    "time"
)

// rechargeSettingsCache 充值配置缓存
type rechargeSettingsCache struct {
    settings  *RechargeSettings
    updatedAt time.Time
}

// SettingService 系统设置服务（扩展）
type SettingService struct {
    settingRepo SettingRepository
    cfg         *config.Config
    onUpdate    func()
    version     string

    // 充值配置缓存
    rechargeCache    *rechargeSettingsCache
    rechargeCacheMu  sync.RWMutex
    rechargeCacheTTL time.Duration

    // 后台任务控制
    stopCh chan struct{}
    wg     sync.WaitGroup
}

const (
    // DefaultRechargeCacheTTL 充值配置缓存有效期（秒）
    DefaultRechargeCacheTTL = 60 * time.Second
)
```

#### 2. 修改构造函数

```go
// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, cfg *config.Config) *SettingService {
    s := &SettingService{
        settingRepo:      settingRepo,
        cfg:              cfg,
        rechargeCacheTTL: DefaultRechargeCacheTTL,
        stopCh:           make(chan struct{}),
    }

    // 启动后台缓存刷新任务
    s.startCacheRefreshLoop()

    return s
}

// startCacheRefreshLoop 启动后台缓存刷新循环
func (s *SettingService) startCacheRefreshLoop() {
    s.wg.Add(1)
    go func() {
        defer s.wg.Done()
        ticker := time.NewTicker(s.rechargeCacheTTL)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                // 定期刷新缓存
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                if err := s.refreshRechargeSettingsCache(ctx); err != nil {
                    log.Printf("[SettingService] Failed to refresh recharge settings cache: %v", err)
                }
                cancel()
            case <-s.stopCh:
                return
            }
        }
    }()
}

// Stop 停止后台任务
func (s *SettingService) Stop() {
    close(s.stopCh)
    s.wg.Wait()
}
```

#### 3. 缓存刷新方法

```go
// refreshRechargeSettingsCache 刷新充值配置缓存
func (s *SettingService) refreshRechargeSettingsCache(ctx context.Context) error {
    settings, err := s.loadRechargeSettingsFromDB(ctx)
    if err != nil {
        return err
    }

    s.rechargeCacheMu.Lock()
    s.rechargeCache = &rechargeSettingsCache{
        settings:  settings,
        updatedAt: time.Now(),
    }
    s.rechargeCacheMu.Unlock()

    return nil
}

// loadRechargeSettingsFromDB 从数据库加载充值配置（内部方法）
func (s *SettingService) loadRechargeSettingsFromDB(ctx context.Context) (*RechargeSettings, error) {
    keys := []string{
        SettingKeyRechargeMinAmount,
        SettingKeyRechargeMaxAmount,
        SettingKeyRechargeDefaultAmounts,
        SettingKeyRechargeOrderExpireMinutes,
    }

    settings, err := s.settingRepo.GetMultiple(ctx, keys)
    if err != nil {
        return nil, fmt.Errorf("get recharge settings from db: %w", err)
    }

    result := &RechargeSettings{
        MinAmount:          DefaultRechargeMinAmount,
        MaxAmount:          DefaultRechargeMaxAmount,
        DefaultAmounts:     DefaultRechargeAmounts,
        OrderExpireMinutes: DefaultRechargeOrderExpireMinutes,
    }

    // 解析逻辑（复用原有代码）
    if raw, ok := settings[SettingKeyRechargeMinAmount]; ok && raw != "" {
        if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
            result.MinAmount = v
        }
    }

    if raw, ok := settings[SettingKeyRechargeMaxAmount]; ok && raw != "" {
        if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
            result.MaxAmount = v
        }
    }

    if raw, ok := settings[SettingKeyRechargeDefaultAmounts]; ok && raw != "" {
        var amounts []float64
        if err := json.Unmarshal([]byte(raw), &amounts); err == nil && len(amounts) > 0 {
            result.DefaultAmounts = amounts
        }
    }

    if raw, ok := settings[SettingKeyRechargeOrderExpireMinutes]; ok && raw != "" {
        if v, err := strconv.Atoi(raw); err == nil && v > 0 {
            result.OrderExpireMinutes = v
        }
    }

    return result, nil
}
```

#### 4. 修改读取方法（优先从缓存读取）

```go
// GetRechargeSettings 获取充值业务配置（优先从缓存读取）
func (s *SettingService) GetRechargeSettings(ctx context.Context) (*RechargeSettings, error) {
    // 尝试从缓存读取
    s.rechargeCacheMu.RLock()
    cache := s.rechargeCache
    s.rechargeCacheMu.RUnlock()

    // 缓存有效
    if cache != nil && time.Since(cache.updatedAt) < s.rechargeCacheTTL {
        // 返回副本，避免外部修改
        return &RechargeSettings{
            MinAmount:          cache.settings.MinAmount,
            MaxAmount:          cache.settings.MaxAmount,
            DefaultAmounts:     append([]float64{}, cache.settings.DefaultAmounts...),
            OrderExpireMinutes: cache.settings.OrderExpireMinutes,
        }, nil
    }

    // 缓存未命中或已过期，从数据库加载
    settings, err := s.loadRechargeSettingsFromDB(ctx)
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

    // 返回副本
    return &RechargeSettings{
        MinAmount:          settings.MinAmount,
        MaxAmount:          settings.MaxAmount,
        DefaultAmounts:     append([]float64{}, settings.DefaultAmounts...),
        OrderExpireMinutes: settings.OrderExpireMinutes,
    }, nil
}
```

#### 5. 修改更新方法（主动失效缓存）

```go
// UpdateRechargeSettings 更新充值业务配置
func (s *SettingService) UpdateRechargeSettings(ctx context.Context, settings *RechargeSettings) error {
    // 验证逻辑（保持不变）
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

    // 保存到数据库
    if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
        return err
    }

    // 主动刷新缓存（确保实时生效）
    s.rechargeCacheMu.Lock()
    s.rechargeCache = &rechargeSettingsCache{
        settings:  settings,
        updatedAt: time.Now(),
    }
    s.rechargeCacheMu.Unlock()

    return nil
}
```

#### 6. 优雅关闭集成

在应用启动/关闭逻辑中（如 `main.go` 或 Wire 注入）：

```go
// 在应用退出时调用
func cleanup(settingService *service.SettingService) {
    settingService.Stop()
}

// 或使用 defer
defer settingService.Stop()
```

### 项目结构对齐

| 文件 | 作用 |
|------|------|
| `backend/internal/service/setting_service.go` | 添加缓存逻辑 |
| `backend/cmd/server/main.go` 或 Wire | 集成优雅关闭 |

### 性能考量

1. **缓存命中率**: 充值配置读取频繁（每次充值页面访问），写入稀少（管理员手动修改），缓存命中率预计 > 99%
2. **内存占用**: 单个 `RechargeSettings` 对象约 100 字节，可忽略不计
3. **并发安全**: 使用 `sync.RWMutex`，读操作使用读锁，写操作使用写锁
4. **一致性**: 主动失效机制保证配置更新后立即可见

### 测试要点

```go
func TestRechargeSettingsCache(t *testing.T) {
    // 测试缓存命中
    t.Run("CacheHit", func(t *testing.T) {
        // 第一次调用从数据库加载
        // 第二次调用应从缓存返回
    })

    // 测试缓存过期
    t.Run("CacheExpired", func(t *testing.T) {
        // 设置较短的 TTL
        // 等待过期后再次调用应从数据库加载
    })

    // 测试更新后缓存失效
    t.Run("CacheInvalidateOnUpdate", func(t *testing.T) {
        // 调用 UpdateRechargeSettings
        // 立即调用 GetRechargeSettings 应返回新值
    })
}
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.4] - 用户故事定义
- [Source: _bmad-output/implementation-artifacts/1-3-admin-recharge-config.md] - 前置 Story
- [Source: backend/internal/service/setting_service.go] - 现有 SettingService 实现

### 设计决策

1. **内存缓存而非 Redis**: 充值配置是轻量级数据，使用进程内缓存更简单可靠
2. **定期刷新 + 主动失效**: 双重保障确保配置及时生效
3. **返回副本**: 避免外部代码意外修改缓存内容
4. **goroutine 后台刷新**: 不阻塞主流程，定期同步数据库变更

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Debug Log References

N/A - 功能已在 Story 1-3 中实现，本 Story 主要补充了单元测试。

### Completion Notes List

1. **功能已实现**：Story 1-4 的所有功能（缓存结构、刷新机制、读取逻辑、主动失效、优雅关闭）已在 Story 1-3 中实现并合并到 setting_service.go
2. **单元测试**：新增 setting_service_test.go，包含 9 个测试用例：
   - TestRechargeSettingsCache_CacheHit: 验证缓存命中
   - TestRechargeSettingsCache_CacheExpired: 验证缓存过期后自动刷新
   - TestRechargeSettingsCache_InvalidateOnUpdate: 验证配置更新后缓存立即失效
   - TestRechargeSettingsCache_DefaultValues: 验证默认值
   - TestRechargeSettingsCache_ReturnsCopy: 验证返回副本（防止外部修改缓存）
   - TestRechargeSettingsCache_ConcurrentAccess: 验证并发安全
   - TestUpdateRechargeSettings_Validation: 验证更新时的参数校验
   - TestSettingService_Stop: 验证优雅关闭
3. **Wire 集成**：SettingService.Stop() 已在 wire.go 的 provideCleanup 中调用

### File List

- backend/internal/service/setting_service_test.go (新增)
