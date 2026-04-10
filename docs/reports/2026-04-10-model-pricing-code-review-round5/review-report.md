# 2026-04-10 Model Pricing 复审报告（Round 5）

## 1. 结论

本轮复审聚焦新加的测试文件 `backend/internal/service/model_pricing_service_test.go`。

**未发现新的 code-review finding。**

这批测试对它们声称覆盖的 service 层行为，基本是成立的：

- `model_key` 在 Create / Update 前会被 lower-case + trim
- 纯空白 `model_key` 会在进入 repo 前被拦成 400
- `enabled=true` 且全 0 价格会返回 400
- 改 key 时旧缓存会清除
- 同 key 更新时缓存会刷新而不是误删
- `LoadCache()` 只加载 enabled 条目
- `Delete()` 的缓存失效和 404 透传行为都被覆盖

**建议状态：APPROVE。**

## 2. 本轮核验点

### 新测试文件本身

已核验文件：

- `backend/internal/service/model_pricing_service_test.go`

测试断言与实现逻辑是对齐的，重点包括：

- `TestCreate_NormalizesModelKeyToLowercase`
- `TestCreate_NormalizesModelKeyTrimsWhitespace`
- `TestUpdate_NormalizesModelKey`
- `TestCreate_BlankModelKeyReturns400`
- `TestUpdate_BlankModelKeyReturns400`
- `TestCreate_ZeroPriceEnabledReturns400`
- `TestCreate_ConflictErrorPassedThrough`
- `TestUpdate_ConflictErrorPassedThrough`
- `TestUpdate_RenameModelKeyEvictsOldCacheEntry`
- `TestUpdate_SameModelKeyDoesNotEvictCache`
- `TestGetCachedPricing_HitReturnsConvertedPricing`
- `TestLoadCache_OnlyLoadsEnabledEntries`
- `TestDelete_EvictsCacheEntry`
- `TestDelete_NotFoundReturns404`

这些用例已经把前几轮报告中提到的 service 层缺口基本补齐。

### 与当前实现的对应关系

- `Create` / `Update` 的调用顺序仍然是先 `normalizeModelKey()`，再 `validatePricingEntry()`：
  - `backend/internal/service/model_pricing_service.go:73`
  - `backend/internal/service/model_pricing_service.go:87`
- 空白 key 在规范化后会直接触发 `MODEL_PRICING_EMPTY_KEY`：
  - `backend/internal/service/model_pricing_service.go:144`
- 改 key 时会先删旧缓存，再写新缓存：
  - `backend/internal/service/model_pricing_service.go:93`
- repository 冲突错误翻译仍然存在：
  - `backend/internal/repository/model_pricing_repo.go:59`
  - `backend/internal/repository/model_pricing_repo.go:80`

## 3. 残余测试缺口

这不是当前测试文件的缺陷，但仍有一个窄范围残余 gap：

- 这批测试是 **service 层 + stub repository** 测试。
- 因此它们不能直接证明：
  - repository 层真的把数据库约束冲突翻译成 `409 MODEL_PRICING_EXISTS`
  - handler / HTTP 响应层真的把 `MODEL_PRICING_EMPTY_KEY` 包装成最终 API envelope

换句话说：

- **service 行为覆盖已经足够好**
- **API 边界行为** 仍然缺少 handler/repository 级别的直接自动化证据

这属于质量护栏缺口，不是当前阻断问题。

## 4. 验证记录

已执行：

```bash
env GOTOOLCHAIN=auto GOSUMDB=sum.golang.org GOPROXY=https://proxy.golang.org,direct \
  go test ./... -run '^$'
```

结果：

- backend 全量编译通过。

已执行（用于确认 unit-tag 测试位于项目既定测试轨道中）：

```bash
cd backend && go test -tags=unit ./...
```

项目文档/脚本中已有对应约定：

- `backend/Makefile`
- `DEV_GUIDE.md`

说明这份 `//go:build unit` 测试文件的放置方式与仓库现有测试体系一致。
