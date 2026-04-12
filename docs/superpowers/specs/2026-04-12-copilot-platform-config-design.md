# Copilot 平台配置 & 菜单重组设计文档

**日期**：2026-04-12  
**状态**：已确认，待实现

---

## 背景

当前问题：
1. `model_mapping` 字段在 Copilot 平台上被错误地用作白名单过滤器（已有代码修复），但 UI 上 mapping 和 whitelist 仍混用同一字段，语义不清
2. Copilot 相关页面分散在不同路由，没有统一的菜单分组
3. 每个 Copilot 账号的参数（max_output_tokens、max_body_kb、model_mapping、model_whitelist）需要逐个手动设置，没有按 plan_type 统一设置默认值的机制

---

## 目标

1. 新增 **Copilot 平台配置页**，按 plan_type 统一设置 4 个参数的默认值
2. 将所有 Copilot 相关页面重组到侧边栏 **Copilot 分组**
3. 在 EditAccountModal 中将 Copilot 的 `model_mapping` 和 `model_whitelist` 分离为两个独立字段

---

## 范围约定

- **只针对 Copilot 平台**，其他平台（OpenAI/Gemini/Anthropic）的 model_mapping/whitelist 逻辑本次不动
- 平台配置是**默认值模板**（A 方案）：账号级配置优先，账号未设置时继承平台配置，平台未设置时用系统默认

---

## Section 1：菜单结构与路由

### 侧边栏新增 Copilot 分组

| 菜单项 | 路由 | 说明 |
|--------|------|------|
| 平台配置 | `/admin/copilot/platform` | 新增页面 |
| 账户列表 | `/admin/copilot/accounts` | 新路由，复用现有账户列表页并预设 platform=copilot 筛选 |
| 成本分析 | `/admin/copilot/cost` | 原 `/admin/copilot/accounts`（`CopilotAccountsView.vue`）改路由 |
| 用户管理 | `/admin/copilot/users` | 原路由，归入分组 |

**路由变更**：
- 原 `/admin/copilot/accounts` → 改为 `/admin/copilot/cost`（无重定向，直接改路由定义）
- 新增 `/admin/copilot/accounts`（账户列表）
- 新增 `/admin/copilot/platform`（平台配置）

---

## Section 2：数据库

### 新表 `copilot_platform_configs`

```sql
CREATE TABLE copilot_platform_configs (
    id                BIGSERIAL PRIMARY KEY,
    plan_type         VARCHAR(32) NOT NULL UNIQUE,
    -- 枚举值: individual_free / individual_pro / individual_pro_plus / business / enterprise

    max_output_tokens BIGINT,      -- Sonnet/Opus 输出 token 上限，NULL 表示不设默认
    max_body_kb       INTEGER,     -- 请求体大小上限 (KB)，NULL 表示不设默认
    model_mapping     JSONB,       -- 模型名称重写 {"from_model": "to_model", ...}
    model_whitelist   JSONB,       -- 允许使用的模型列表 ["model-a", "model-b"]

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 预插入 5 行（全字段 NULL），确保后端始终能查到记录
INSERT INTO copilot_platform_configs (plan_type) VALUES
    ('individual_free'),
    ('individual_pro'),
    ('individual_pro_plus'),
    ('business'),
    ('enterprise');
```

### Ent Schema

新增 `CopilotPlatformConfig` schema，字段与表结构一致，`plan_type` 加唯一索引。

### 继承逻辑（运行时三层优先级）

```
1. 账号级配置（credentials 字段有值）→ 优先使用
2. 平台配置（copilot_platform_configs 对应 plan_type 行）→ 账号级为空时使用
3. 系统默认（现有硬编码逻辑）→ 两者都没有时使用
```

继承逻辑落点：`CopilotGatewayService` 中现有的参数读取调用（`GetCredential("copilot_max_output_tokens")` 等）之后加 fallback 查询。

---

## Section 3：后端 API

### 新增接口

挂载在 `/api/v1/admin/copilot/platform-config`：

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/admin/copilot/platform-config` | 获取全部 5 个 plan_type 的配置 |
| `PUT` | `/api/v1/admin/copilot/platform-config/:plan_type` | 更新指定 plan_type 的配置 |

**GET 响应**（固定返回 5 条，按 plan_type 顺序）：
```json
[
  {
    "plan_type": "individual_free",
    "max_output_tokens": null,
    "max_body_kb": null,
    "model_mapping": {},
    "model_whitelist": []
  }
]
```

**PUT 请求体**（所有字段可选，null 表示清除）：
```json
{
  "max_output_tokens": 8192,
  "max_body_kb": 400,
  "model_mapping": {"claude-sonnet-4-5": "claude-sonnet-4.6"},
  "model_whitelist": ["claude-sonnet-4.6", "gpt-4o"]
}
```

### 新增组件

- `CopilotPlatformConfigRepo`（repository 层）：GetAll / Upsert by plan_type
- `CopilotPlatformConfigService`（service 层）：GetAll / UpdateByPlanType / GetByPlanType（供 gateway 继承逻辑调用）
- `CopilotPlatformConfigHandler`（handler 层）：List / Update
- 路由注册到 `admin.go` 的 copilot 分组

### model_whitelist 账号选择逻辑

在 `isModelSupportedByAccount` 的 Copilot 分支（今天已修复，始终 return true）基础上：

```
Copilot 账号选账号时：
  1. 取账号级 model_whitelist（credentials.model_whitelist）
  2. 若为空，取平台配置对应 plan_type 的 model_whitelist
  3. 若白名单非空，请求模型必须在白名单内 → 否则 filteredModelMapping++
  4. 若白名单为空 → return true（允许所有，现有行为）
```

**注意**：继承逻辑需要知道账号的 plan_type，通过 `account.GetCredential("plan_type")` 获取。

---

## Section 4：前端

### 4.1 新文件

| 文件 | 说明 |
|------|------|
| `src/views/admin/copilot/CopilotPlatformConfigView.vue` | 平台配置页 |
| `src/views/admin/copilot/CopilotAccountListView.vue` | Copilot 账户列表页（复用现有组件，预设筛选） |
| `src/api/admin/copilotPlatformConfig.ts` | 平台配置 API 调用层 |

**路由变更文件**：`src/router/index.ts`  
**侧边栏变更文件**：`src/components/layout/AppSidebar.vue`  
**i18n 新增**：`src/i18n/locales/zh.ts` + `en.ts`

### 4.2 平台配置页布局

`CopilotPlatformConfigView.vue`：

- 页面顶部：标题 + 说明文字（"为各 plan 类型设置参数默认值，账号级配置优先"）
- 5 张卡片（`plan_type` 各一张），每张卡片：
  - **标题**：Free / Pro / Pro+ / Business / Enterprise
  - **max_output_tokens**：数字输入框，空值=不设默认
  - **max_body_kb**：数字输入框，空值=不设默认
  - **模型映射**：复用 EditAccountModal 里的 mapping 多行编辑器组件（抽取为独立组件 `CopilotModelMappingEditor.vue`）
  - **模型白名单**：复用 `ModelWhitelistSelector.vue`，platform 固定为 `copilot`
  - **保存按钮**：每张卡片独立保存（PUT 单个 plan_type），保存中显示 loading，成功显示 toast

### 4.3 EditAccountModal 分离 whitelist/mapping

Copilot 账号编辑表单（`account.platform === 'copilot'` 区块）：

**现有**（模型映射区块）：保持不变，只做名称重写

**新增**（模型映射下方，独立区块）：
```
Copilot 模型白名单
[说明文字：只有白名单内的模型才会被路由到此账号，留空允许所有模型]
<ModelWhitelistSelector v-model="copilotModelWhitelist" platform="copilot" />
```

读取：`credentials.model_whitelist`（字符串数组）  
保存：与现有 credentials 字段一起提交到 UpdateAccount API

---

## 实现顺序

1. **DB 迁移**：新建表 + 预插入 5 行
2. **Ent schema**：生成 CopilotPlatformConfig 实体
3. **后端**：Repo → Service → Handler → 路由注册
4. **继承逻辑**：CopilotGatewayService 的 3 个参数读取点加 fallback
5. **model_whitelist 账号选择**：isModelSupportedByAccount Copilot 分支加白名单检查
6. **前端路由 & 侧边栏**：重组 Copilot 分组
7. **CopilotPlatformConfigView**：平台配置页
8. **EditAccountModal**：新增白名单字段
