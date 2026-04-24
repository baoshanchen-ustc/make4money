#!/usr/bin/env bash
# =============================================================================
# Sub2API One-Shot Bootstrap
# =============================================================================
# 零参数部署：第一次运行自动生成所有 secrets，后续幂等。
#
# 用法：
#     cd deploy && ./quickstart.sh           # 启动 / 升级
#     cd deploy && ./quickstart.sh down      # 停止
#     cd deploy && ./quickstart.sh logs -f   # 看日志
#
# 第一次完成后：
#     浏览器访问 http://<服务器 IP>:8080
#     用脚本输出的 admin 邮箱 + 密码登录
#     左侧「账号管理」→ 添加 OpenAI Codex / GPT Plus 订阅账号
#     左侧「分组管理」→ 直接保存（默认已为 Claude Code 人设预设）
#     生成 API Key → claude code CLI 配置该 Key 即可
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ENV_FILE="${SCRIPT_DIR}/.env"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"

# 检查依赖
if ! command -v docker >/dev/null 2>&1; then
    echo "❌ 未找到 docker。请先安装 Docker Engine: https://docs.docker.com/engine/install/" >&2
    exit 1
fi
if ! docker compose version >/dev/null 2>&1; then
    echo "❌ 未找到 docker compose v2 (docker compose 命令)。请升级 Docker。" >&2
    exit 1
fi

# 优先选最稳定的随机源
randhex() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "$1"
    elif [ -r /dev/urandom ]; then
        head -c $(( $1 )) /dev/urandom | od -An -vtx1 | tr -d ' \n' | head -c $(( $1 * 2 ))
    else
        echo "❌ 无可用随机源（openssl 或 /dev/urandom）" >&2
        exit 1
    fi
}
randstr() {
    # 长度 N、url-safe 字符（去掉 / + =）
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 $(( $1 * 2 )) | tr -d '/+=\n' | head -c "$1"
    else
        head -c $(( $1 * 2 )) /dev/urandom | base64 | tr -d '/+=\n' | head -c "$1"
    fi
}

# 第一次：生成 .env
if [ ! -f "${ENV_FILE}" ]; then
    echo ">>> 第一次部署：生成强随机 secrets 到 ${ENV_FILE}"
    PG_PWD=$(randstr 32)
    JWT=$(randhex 48)
    TOTP=$(randhex 32)
    ADMIN_PWD=$(randstr 20)
    cat > "${ENV_FILE}" <<EOF
# Sub2API 自动生成于 $(date -u +%Y-%m-%dT%H:%M:%SZ)
# 含敏感信息，已在 .gitignore 中排除。请妥善备份。
# 可手动编辑后重启容器使新值生效。

# ---- 监听地址 ----
BIND_HOST=0.0.0.0
SERVER_PORT=8080

# ---- PostgreSQL ----
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=${PG_PWD}
POSTGRES_DB=sub2api

# ---- Redis ----
REDIS_PASSWORD=
REDIS_DB=0

# ---- 管理员账号（首次启动注入） ----
ADMIN_EMAIL=admin@sub2api.local
ADMIN_PASSWORD=${ADMIN_PWD}

# ---- 安全密钥（变更将使既有 JWT / TOTP 失效） ----
JWT_SECRET=${JWT}
TOTP_ENCRYPTION_KEY=${TOTP}

# ---- 运行模式 ----
RUN_MODE=standard
SERVER_MODE=release
TZ=Asia/Shanghai
EOF
    chmod 600 "${ENV_FILE}" 2>/dev/null || true
    echo ">>> .env 已生成（chmod 600）"
fi

# 透传子命令；默认 up -d
ACTION="${1:-up}"
shift || true

case "${ACTION}" in
    up|start)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d "$@"
        echo ""
        echo "============================================================"
        echo " ✅ Sub2API 已启动"
        echo "============================================================"
        ADMIN_EMAIL=$(grep -E '^ADMIN_EMAIL=' "${ENV_FILE}" | cut -d= -f2-)
        ADMIN_PASSWORD=$(grep -E '^ADMIN_PASSWORD=' "${ENV_FILE}" | cut -d= -f2-)
        SERVER_PORT=$(grep -E '^SERVER_PORT=' "${ENV_FILE}" | cut -d= -f2-)
        echo " 访问地址 : http://<本机 IP>:${SERVER_PORT:-8080}"
        echo " 管理邮箱 : ${ADMIN_EMAIL}"
        echo " 管理密码 : ${ADMIN_PASSWORD}"
        echo "------------------------------------------------------------"
        echo " 下一步："
        echo "   1) 浏览器登录"
        echo "   2) 「账号管理」→ 添加 OpenAI Codex / GPT Plus 账号"
        echo "   3) 「分组管理」→ 新建（默认已是 Claude Code 人设）"
        echo "   4) 把账号绑定到该分组 → 生成 API Key"
        echo "   5) Claude Code CLI 配置该 Key 即可使用"
        echo "============================================================"
        ;;
    down|stop)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" down "$@"
        ;;
    logs)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" logs "$@"
        ;;
    restart)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" restart "$@"
        ;;
    pull)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" pull "$@"
        ;;
    ps|status)
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" ps "$@"
        ;;
    *)
        # 透传任意其他 docker compose 子命令
        docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" "${ACTION}" "$@"
        ;;
esac
