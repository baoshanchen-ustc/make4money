# Sub2API · Zeabur 部署手册

针对 [Zeabur](https://zeabur.com) PaaS 的部署指南。无需 SSH、无需写 docker-compose、无需配 HTTPS——平台代办。

---

## 0. 部署前清单

- 一个 GitHub 账号，仓库 fork 到自己名下（或镜像到私仓也行）
- 一个 Zeabur 账号（可用 GitHub 登录）
- （可选）自定义域名 + DNS 控制权

---

## 1. 创建项目和服务

在 Zeabur 控制台：

1. **New Project** → 选区域（`hkg` 香港 / `sjc` 美西，按你的客户端就近选）
2. **Add Service** 三次，依次添加：
   - **Postgres**（Marketplace → PostgreSQL，版本选 17+）
   - **Redis**（Marketplace → Redis，版本选 7+）
   - **Sub2API**（Git → 选你的仓库 → 分支 `main` → 自动识别 Dockerfile）

> 三个服务在同一个 Project 内，Zeabur 会自动建立内网，不暴露公网端口。

---

## 2. 注入环境变量

打开 Sub2API 服务 → **Variables** → **Raw Editor** → 粘贴 [`deploy/zeabur.env.template`](./zeabur.env.template) 全文。

然后**填三处空值**：

```bash
# 在你本地终端跑这三条，把结果填到面板对应字段
openssl rand -hex 32        # → JWT_SECRET
openssl rand -hex 32        # → TOTP_ENCRYPTION_KEY
openssl rand -base64 24     # → ADMIN_PASSWORD
```

> ⚠️ `TOTP_ENCRYPTION_KEY` 立刻另存到密码管理器。丢了所有用户 2FA 失效。

`${POSTGRES_HOST}` / `${REDIS_HOST}` 这种引用变量会**自动解析**到同 Project 的对应服务，**不要手动改**。

---

## 3. 配置端口与 Volume

### 端口

Sub2API 服务 → **Network** → **Add Port**：
- Name: `web`
- Port: `8080`
- 勾选 **Public**（启用后 Zeabur 自动分配 `<service>.zeabur.app` 域名 + HTTPS）

### Volume（必须）

Sub2API 服务 → **Volumes** → **Add Volume**：
- Mount Path: `/app/data`
- Size: `1` GB（起步够用，后续可扩）

> 不挂 Volume 的后果：`logs/`、`config.yaml`、`model_pricing.json` 在容器重启时全部丢失。

Postgres / Redis 用的是 Zeabur 托管服务，自带持久化，不用单独配。

---

## 4. 构建与首次启动

### 构建参数（可选，海外节点优化）

Sub2API 服务 → **Settings** → **Build** → **Build Args**：

```
GOPROXY=https://proxy.golang.org,direct
GOSUMDB=sum.golang.org
```

仓库 Dockerfile 默认指向国内镜像（`goproxy.cn`），Zeabur 海外构建机用国外源更稳。如果你部署在 `hkg` 区域且发现拉模块慢，再加。

### 触发部署

push 一次 `main` 分支，或在 Sub2API 服务面板按 **Redeploy**。

构建时长：首次约 5-10 分钟（多阶段构建：node + go + alpine）。

### 首次启动检查

部署成功后：

```
1. 访问 https://<service>.zeabur.app/health → 应返回 {"status":"ok"}
2. 访问 https://<service>.zeabur.app/      → 进入登录页
3. 用 ADMIN_EMAIL + ADMIN_PASSWORD 登录
4. 立刻进设置改管理员密码（不要长期依赖环境变量里的密码）
```

---

## 5. 自定义域名

Sub2API 服务 → **Network** → **Domains** → **Add Domain**：

1. 填 `api.yourdomain.com`
2. Zeabur 显示一个 CNAME 目标，去 DNS 商加一条 CNAME
3. DNS 生效后（一般几分钟）Zeabur 自动签 Let's Encrypt 证书

---

## 6. 升级流程

```bash
# 本地拉取上游更新
git pull upstream main
git push origin main

# Zeabur 自动检测 main 分支变化并触发 redeploy
# 或在 Sub2API 服务面板手动按 Redeploy
```

升级期间约 30-90 秒短暂中断。Zeabur 暂不支持蓝绿，需要无中断的话需要双实例 + 外部 LB。

---

## 7. 备份策略

Zeabur 托管 Postgres 提供基础备份，但**强烈建议**自建定时备份到对象存储：

### 方案 A：Zeabur Cron Service（推荐）

Add Service → **Cron** → 用一个最小镜像跑 `pg_dump`：

```yaml
# 示例（在 Zeabur Cron 服务的 Command 字段）
schedule: "0 3 * * *"
command: |
  pg_dump "postgresql://${POSTGRES_USERNAME}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DATABASE}" \
  | gzip \
  | aws s3 cp - s3://your-bucket/sub2api/$(date +%Y%m%d).sql.gz
```

### 方案 B：本地拉取

定时从你本机运行：

```bash
psql_url=$(zeabur env get POSTGRES_URL --service postgres)
pg_dump "$psql_url" | gzip > sub2api-$(date +%Y%m%d).sql.gz
```

---

## 8. 常见问题

| 现象 | 原因 | 解决 |
|---|---|---|
| 部署成功但 `/health` 502 | 容器没起来 | 看 **Logs** 面板，多半是数据库连接失败 → 确认 Postgres 服务已 Healthy |
| 启动报 `failed to migrate` | DB 用户权限不够 | Zeabur Postgres 默认用户已是 owner，检查 DATABASE_USER 是否引用对了变量 |
| `TOTP_ENCRYPTION_KEY length must be 32 bytes` | 密钥不是 hex64 | 用 `openssl rand -hex 32`（输出 64 字符 = 32 字节） |
| 前端 404 | Dockerfile 构建阶段没嵌入前端 | 看构建日志确认 `pnpm build` 成功 |
| 拉镜像/模块超时 | 海外节点用国内源 | 设置 Build Args `GOPROXY=https://proxy.golang.org,direct` |
| 升级后管理员密码失效 | `ADMIN_PASSWORD` 仅首次启动注入，改密码靠管理界面 | 进控制台改密 |

---

## 9. Zeabur vs 自建 VPS 取舍

| 维度 | Zeabur | 自建 VPS |
|---|---|---|
| 上手速度 | 5 分钟 | 1-2 小时 |
| 月成本（小流量） | $5-15 | $5-10（VPS 自己） |
| 月成本（中等流量） | $30-80 | $10-30 |
| HTTPS / 域名 | 自动 | 需配 Caddy/Nginx + Let's Encrypt |
| 水平扩展 | 面板加副本 | 自己搭 LB |
| 数据库可控性 | 受限（黑盒） | 完全可控 |
| 备份/迁移 | 跟着 Zeabur 走 | 完全自主 |
| 网络出海 | 看节点区域 | 看 VPS 机房 |

**建议**：
- 自用 / 小团队 → Zeabur 省心
- 流量大 / 需要细调 / 数据合规 → 自建 VPS（用 `deploy/docker-compose.yml`）
