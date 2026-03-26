# Oracle Operations

本文件记录 Oracle 生产机 `/home/ubuntu/sub2api` 的最低限度运维约定，目标是把发布、备份、自检和 GitHub 访问都固化到仓库和主机配置里，避免再次出现“远端分支已变更，但生产机本地提交链对不上”的补救动作。

## 当前文件

- `deploy/Caddyfile.oracle-a1-free`
  - Oracle 当前生效的 Caddy 配置快照，来源于 `/etc/caddy/Caddyfile`
- `deploy/ops/preflight_oracle.sh`
  - 发布前/巡检前自检脚本
- `deploy/ops/backup_sub2api.sh`
  - 在线备份脚本，不停止服务
- `deploy/ops/verify_backup_sub2api.sh`
  - 备份完整性检查脚本
- `deploy/ops/release_oracle.sh`
  - 标准化 Oracle 发布脚本
- `deploy/ops/sub2api`
  - Oracle 主机命令入口，可通过 `sub2api update` 等子命令调用标准运维流程
- `deploy/systemd/sub2api-backup.service`
- `deploy/systemd/sub2api-backup.timer`

## GitHub 访问

Oracle 主机当前通过专用 SSH key 访问 GitHub：

- key 路径：`/home/ubuntu/.ssh/id_ed25519_sub2api_github`
- SSH config：`/home/ubuntu/.ssh/config`
- 当前仓库 remote：
  - `fork -> git@github.com:isjiajia01/sub2api.git`
  - `origin -> git@github.com:Wei-Shaw/sub2api.git`

常用检查：

```bash
ssh -T git@github.com
gh auth status
gh pr status
```

## 标准发布流程

推荐顺序：

1. 在本地开发机完成修改并推到 `fork/fix/openai-system-message-lifting`
2. 登录 Oracle 主机后执行：

```bash
sub2api update
```

`deploy/ops/release_oracle.sh` 会做这些事：

- 要求当前分支就是 `fix/openai-system-message-lifting`
- 要求工作区干净
- 要求 `deploy/docker-compose.yml` 仍然是仓库源码构建（`build: ..` + `image: sub2api-local:latest`）
- `git fetch fork fix/openai-system-message-lifting`
- 如果本地有未推送提交，直接失败，而不是继续部署
- 默认先跑一次在线备份
- 仅对 `sub2api` 服务执行 `docker compose up -d --build --no-deps`
- 部署后等待 `/healthz` 恢复，再检查运行镜像必须是 `sub2api-local:latest`，最后重新跑 preflight

常用参数：

```bash
BACKUP_BEFORE_DEPLOY=0 bash deploy/ops/release_oracle.sh
DEPLOY_IF_UP_TO_DATE=1 bash deploy/ops/release_oracle.sh
REMOTE=fork BRANCH=fix/openai-system-message-lifting bash deploy/ops/release_oracle.sh
```

`sub2api` 命令常用子命令：

```bash
sub2api update
sub2api preflight
sub2api backup
sub2api verify-backup
sub2api status
sub2api version
```

不要再用下面这类旧命令做生产更新：

```bash
sudo docker compose pull sub2api && sudo docker compose up -d sub2api
```

这会把生产机切回官方镜像，绕开 Oracle 本地兼容补丁。

## 备份

每次备份会写入 `/home/ubuntu/backups/sub2api/<UTC timestamp>/`，至少包含：

- `postgres.dump`
- `sub2api_data.tgz`
- `redis_data.tgz`
- `.env`
- `Caddyfile.oracle-live`
- `manifest.txt`
- `SHA256SUMS`（如果系统可用）

常用命令：

```bash
cd /home/ubuntu/sub2api
bash deploy/ops/backup_sub2api.sh
bash deploy/ops/verify_backup_sub2api.sh latest
sudo systemctl status sub2api-backup.timer
```

## 运维约束

- 不要直接把生产机仓库当开发机长期改代码；先在本地/分支完成，再推到 fork，再在 Oracle 上 fast-forward 发布。
- 生产发布唯一入口是 `sub2api update`（底层仍调用 `deploy/ops/release_oracle.sh`）；不要再手动执行 `docker compose pull sub2api`。
- 每次改 `/etc/caddy/Caddyfile` 后，同步回 `deploy/Caddyfile.oracle-a1-free`，避免继续漂移。
- `sub2api-backup.timer` 只负责产出备份，不自动删除旧备份。清理前先人工确认。
- 发布前后至少跑一次 `deploy/ops/preflight_oracle.sh`。
- 如果 `deploy/ops/release_oracle.sh` 提示本地分支含有 remote 上没有的提交，不要继续在生产机上手改；先把变更 push 到 fork，再回来发布。
