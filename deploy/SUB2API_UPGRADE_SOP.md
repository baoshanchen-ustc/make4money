# sub2api 升级 SOP

## 结论

升级 Oracle 上的 `sub2api` 时，默认走这条路径：

1. 先打回退点
2. 只读拉取上游 `origin/main`
3. 在当前线上分支上做 `merge`
4. 只处理必要冲突，不直接切主线
5. `docker compose up -d --build`
6. 做登录态 API 冒烟测试
7. 把合并结果推到 `fork`

不要直接在 Oracle 上硬切 `origin/main`。

---

## 当前约定

服务器路径：

- 仓库：`/home/ubuntu/sub2api`
- 部署目录：`/home/ubuntu/sub2api/deploy`

Git 远端策略：

- `origin`：`https://github.com/Wei-Shaw/sub2api.git`（fetch）
- `origin`：`disabled://origin-readonly`（push）
- `fork`：`https://github.com/isjiajia01/sub2api.git`（fetch）
- `fork`：`git@github.com:isjiajia01/sub2api.git`（push）

检查命令：

```bash
cd /home/ubuntu/sub2api
git remote -v
```

---

## 升级前检查

先确认当前线上状态：

```bash
cd /home/ubuntu/sub2api
git branch --show-current
git rev-parse HEAD
git status --short
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' | grep sub2api
```

要求：

- 工作区必须干净
- 当前线上分支必须明确
- `sub2api` / `postgres` / `redis` 都正常运行

---

## 1. 打回退点

升级前先在 Oracle 上打 tag：

```bash
cd /home/ubuntu/sub2api
git tag -f pre-upgrade-$(date +%Y%m%d-%H%M) HEAD
```

如果升级失败，可直接回退：

```bash
cd /home/ubuntu/sub2api
git reset --hard <回退tag或commit>
cd /home/ubuntu/sub2api/deploy
docker compose up -d --build
```

---

## 2. 获取上游最新主线

Oracle 上不要依赖 SSH 拉上游，直接用 HTTPS：

```bash
cd /home/ubuntu/sub2api
git fetch https://github.com/Wei-Shaw/sub2api.git main
git rev-parse FETCH_HEAD
git rev-list --left-right --count HEAD...FETCH_HEAD
```

说明：

- `FETCH_HEAD` 就是上游最新 `main`
- `rev-list --left-right --count` 用来判断双方各自领先多少提交

---

## 3. 在当前线上分支上合并最新主线

推荐：

```bash
cd /home/ubuntu/sub2api
git merge --no-ff --no-commit FETCH_HEAD
```

为什么用 `merge`：

- 比 `rebase` 更稳
- 更容易保留线上分支的历史语义
- 冲突处理后回退简单

如果无冲突：

```bash
git commit -m "merge: integrate upstream main into <current-branch>"
```

如果有冲突：

- 只处理冲突文件
- 优先保留线上已验证过的 Oracle 定制逻辑
- 不顺手做无关重构

处理完：

```bash
git add <resolved-files>
git commit -m "merge: integrate upstream main into <current-branch>"
```

---

## 4. 重建并替换容器

```bash
cd /home/ubuntu/sub2api/deploy
docker compose up -d --build
```

说明：

- 旧容器会一直跑到新镜像构建完成
- 大版本升级时，前端 `pnpm build` 和后端 `go build` 会比较慢
- 正常情况下，服务不会在构建阶段中断

查看构建状态：

```bash
docker ps --format 'table {{.Names}}\t{{.RunningFor}}\t{{.Status}}'
docker logs --tail=50 sub2api
ps -eo pid,etime,pcpu,pmem,command | grep -E 'go build|docker build|buildkit|pnpm run build'
```

---

## 5. 最小验收

### 容器健康

```bash
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}' | grep sub2api
docker logs --tail=50 sub2api
```

要求：

- `sub2api` 显示 `healthy`
- 日志中看到 `Server started on 0.0.0.0:8080`

### 外层入口

```bash
curl -k -I -sS https://api.zhangjiajia.me/
```

要求：

- 返回 `200` 或符合预期的页面响应

### 登录态 API 冒烟

```bash
ADMIN_PASSWORD=$(grep '^ADMIN_PASSWORD=' /home/ubuntu/sub2api/deploy/.env | cut -d= -f2-)

TOKEN=$(curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"admin@sub2api.local\",\"password\":\"$ADMIN_PASSWORD\"}" \
  | python3 -c 'import sys,json; d=json.load(sys.stdin); print(d.get("data",{}).get("access_token",""))')

curl -sS http://127.0.0.1:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN"

curl -sS http://127.0.0.1:8080/health -o /dev/null -w 'HEALTH_HTTP=%{http_code}\n'
```

要求：

- 登录成功
- `/api/v1/auth/me` 返回管理员信息
- `/health` 返回 `200`

---

## 6. 升级后同步到 fork

Oracle 不应成为唯一事实来源。

如果 Oracle 上没有可用 GitHub push 权限：

1. 在 Oracle 上打 bundle
2. 拉回本机
3. 用本机 GitHub 凭据推到 `fork`

Oracle 上生成完整 bundle：

```bash
cd /home/ubuntu/sub2api
git bundle create /tmp/sub2api-upgrade.bundle HEAD
```

然后在本机导入并推送到：

- `fork/fix/openai-system-message-lifting`

---

## 7. 常见坑

### 1. `origin` 被写成可 push

修正：

```bash
cd /home/ubuntu/sub2api
git remote set-url origin https://github.com/Wei-Shaw/sub2api.git
git remote set-url --push origin disabled://origin-readonly
```

### 2. Oracle 上 `git push fork` 失败

常见原因：

- 没有 GitHub SSH key
- 没装 `gh`
- 只有 fetch 权限，没有 push 权限

解决：

- 推送走本机
- Oracle 只负责构建与运行

### 3. 构建时间太长

这是正常现象，尤其是：

- `pnpm install`
- `vite build`
- `go build`

不要因为旧容器还在跑就误判失败。

### 4. `/health` 或匿名 API 路由行为变化

升级后不要只盯某一个旧接口路径。
优先验证：

- 容器健康
- 启动日志
- 登录态 API

---

## 8. 本次升级参考

一次已验证路径：

- 升级前：`50ba9063821457669b628e48c84258e636f5cd0a`
- 上游主线：`0236b97d496e8d5a4bd56b73ad2fa29aa56fba10`
- 合并后：`70b28a540d1c4a46852b841d739547ea5a19a656`

回退点：

- `pre-upgrade-20260319-1`
