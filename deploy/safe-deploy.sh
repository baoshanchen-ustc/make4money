#!/bin/bash
# =============================================================================
# Sub2API 零停机安全部署脚本 (Blue-Green Zero-Downtime Deploy)
# =============================================================================
# 原理：
#   Nginx 作为用户入口，后面有 blue / green 两个 Sub2API 实例。
#   部署时启动新实例，验收通过后 Nginx 切流量，旧实例优雅下线。
#   用户全程无感知，任何验收失败都不会影响线上。
#
# 流程：
#   1. 拉取新镜像
#   2. 启动备用实例（如果当前是 blue，就启动 green）
#   3. 对备用实例做 4 阶段验收检查
#   4. 验收通过 → Nginx 切换流量到新实例 → 停掉旧实例
#   5. 验收失败 → 停掉备用实例 → 线上完全不受影响
#
# 用法：
#   ./safe-deploy.sh              # 零停机部署
#   ./safe-deploy.sh --rollback   # 回滚到上一实例
#   ./safe-deploy.sh --status     # 查看当前状态
#   ./safe-deploy.sh --logs       # 查看部署日志
#   ./safe-deploy.sh --cron 5     # 每 5 分钟自动检查更新
#   ./safe-deploy.sh --init       # 首次初始化（启动全部基础设施）
# =============================================================================

set -euo pipefail

# ===================== 配置 =====================
DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_FILE="docker-compose.bluegreen.yml"
IMAGE_NAME="weishaw/sub2api"
STATE_FILE="${DEPLOY_DIR}/.deploy-state"
LOG_FILE="${DEPLOY_DIR}/deploy.log"
NGINX_UPSTREAM="${DEPLOY_DIR}/nginx/upstream.conf"

# 健康检查配置
MAX_WAIT=120          # 最长等待秒数
CHECK_INTERVAL=3      # 每次检查间隔秒数
GRACEFUL_WAIT=10      # 切换后等旧连接排空的秒数

# ===================== 颜色 =====================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ===================== 日志 =====================
log() {
    local level="$1"
    shift
    local msg="$*"
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[${timestamp}] [${level}] ${msg}" >> "${LOG_FILE}"

    case "$level" in
        INFO)    echo -e "${BLUE}[${timestamp}]${NC} ${msg}" ;;
        OK)      echo -e "${GREEN}[${timestamp}] ✓${NC} ${msg}" ;;
        WARN)    echo -e "${YELLOW}[${timestamp}] ⚠${NC} ${msg}" ;;
        ERROR)   echo -e "${RED}[${timestamp}] ✗${NC} ${msg}" ;;
        STEP)    echo -e "${CYAN}[${timestamp}] ▶${NC} ${msg}" ;;
    esac
}

# ===================== 状态管理 =====================

# 读取当前活跃的 slot (blue 或 green)
get_active_slot() {
    if [ -f "${STATE_FILE}" ]; then
        cat "${STATE_FILE}"
    else
        echo "blue"
    fi
}

# 获取备用 slot
get_standby_slot() {
    local active
    active=$(get_active_slot)
    if [ "$active" = "blue" ]; then
        echo "green"
    else
        echo "blue"
    fi
}

# 保存活跃 slot
set_active_slot() {
    echo "$1" > "${STATE_FILE}"
}

# 获取容器名
container_name() {
    echo "sub2api-${1}"
}

# ===================== 辅助函数 =====================

is_container_running() {
    local status
    status=$(docker inspect -f '{{.State.Status}}' "$1" 2>/dev/null || echo "not_found")
    [ "$status" = "running" ]
}

# 获取容器在 Docker 网络中的 IP
get_container_ip() {
    docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$1" 2>/dev/null || echo ""
}

# ===================== 健康检查（对指定容器） =====================

# 对指定容器直接做 HTTP 健康检查（不经过 Nginx）
# 优先通过容器 IP 直连，如果获取不到 IP 则用 docker exec 回退
check_container_health() {
    local container="$1"
    local ip
    ip=$(get_container_ip "$container")

    log STEP "阶段 1/4: 等待容器 ${container} 启动 (最长 ${MAX_WAIT}s)"

    local elapsed=0
    local http_code="000"
    while [ $elapsed -lt $MAX_WAIT ]; do
        if [ -n "$ip" ]; then
            # 优先：通过容器 IP 直连检查
            http_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 --max-time 5 "http://${ip}:8080/health" 2>/dev/null || echo "000")
        else
            # 回退：通过 docker exec 在容器内检查
            http_code=$(docker exec "$container" curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 --max-time 5 "http://localhost:8080/health" 2>/dev/null || echo "000")
        fi

        if [ "$http_code" = "200" ]; then
            log OK "容器 ${container} HTTP 健康检查通过 (${elapsed}s)"
            return 0
        fi

        elapsed=$((elapsed + CHECK_INTERVAL))
        if [ $elapsed -lt $MAX_WAIT ]; then
            printf "\r  等待中... %ds/%ds (HTTP %s)" "$elapsed" "$MAX_WAIT" "$http_code"
            sleep $CHECK_INTERVAL
        fi
    done
    echo ""
    log ERROR "容器 ${container} 在 ${MAX_WAIT}s 内未响应 (最后状态码: ${http_code})"
    return 1
}

# 检查基础设施（PostgreSQL + Redis）
check_infra() {
    log STEP "阶段 2/4: 基础设施检查"

    local all_ok=true

    local pg_health
    pg_health=$(docker inspect -f '{{.State.Health.Status}}' sub2api-postgres 2>/dev/null || echo "unknown")
    if [ "$pg_health" = "healthy" ]; then
        log OK "PostgreSQL: healthy"
    else
        log ERROR "PostgreSQL: ${pg_health}"
        all_ok=false
    fi

    local redis_health
    redis_health=$(docker inspect -f '{{.State.Health.Status}}' sub2api-redis 2>/dev/null || echo "unknown")
    if [ "$redis_health" = "healthy" ]; then
        log OK "Redis: healthy"
    else
        log ERROR "Redis: ${redis_health}"
        all_ok=false
    fi

    $all_ok
}

# 对指定容器做 API 烟雾测试
check_smoke_test() {
    local container="$1"
    local ip
    ip=$(get_container_ip "$container")
    local base_url
    if [ -n "$ip" ]; then
        base_url="http://${ip}:8080"
    else
        base_url="http://localhost:8080"
    fi

    log STEP "阶段 3/4: API 烟雾测试 (${container})"

    local all_ok=true

    # 辅助函数：根据是否有 IP 选择 curl 方式
    _smoke_curl() {
        if [ -n "$ip" ]; then
            curl "$@" 2>/dev/null || echo "000"
        else
            docker exec "$container" curl "$@" 2>/dev/null || echo "000"
        fi
    }

    # 测试 1: /setup/status 可达
    local setup_code
    setup_code=$(_smoke_curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 "${base_url}/setup/status")
    if [ "$setup_code" = "200" ]; then
        log OK "/setup/status 可达 (HTTP ${setup_code})"
    else
        log WARN "/setup/status 返回 HTTP ${setup_code}"
    fi

    # 测试 2: 登录接口可达（发空请求，期望 4xx 而不是 5xx）
    local login_code
    login_code=$(_smoke_curl -s -o /dev/null -w "%{http_code}" \
        --connect-timeout 5 --max-time 10 \
        -X POST "${base_url}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{}')

    if [ "$login_code" -ge 200 ] 2>/dev/null && [ "$login_code" -lt 500 ] 2>/dev/null; then
        log OK "登录接口可达 (HTTP ${login_code})"
    elif [ "$login_code" -ge 500 ] 2>/dev/null; then
        log ERROR "登录接口返回服务器错误 (HTTP ${login_code})"
        all_ok=false
    else
        log WARN "登录接口状态: HTTP ${login_code}"
    fi

    # 测试 3: 检查容器日志中的严重错误
    local error_count
    error_count=$(docker logs "$container" --since 60s 2>&1 | grep -ciE 'panic|fatal|segfault' || echo "0")
    if [ "$error_count" -eq 0 ]; then
        log OK "最近 60s 无 panic/fatal 错误"
    else
        log ERROR "发现 ${error_count} 条 panic/fatal 错误"
        all_ok=false
    fi

    $all_ok
}

# 通过 Nginx 验证流量切换成功
check_nginx_routing() {
    local port="${SERVER_PORT:-8080}"
    local url="http://localhost:${port}/health"

    log STEP "阶段 4/4: Nginx 流量验证"

    local http_code
    http_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 "${url}" 2>/dev/null || echo "000")

    if [ "$http_code" = "200" ]; then
        log OK "Nginx 转发正常 (HTTP ${http_code})"
        return 0
    else
        log ERROR "Nginx 转发异常 (HTTP ${http_code})"
        return 1
    fi
}

# 完整验收流程（对指定容器）
run_verification() {
    local container="$1"
    log INFO "========== 对 ${container} 执行验收检查 =========="

    local passed=0
    local failed=0

    # 阶段 1: HTTP 健康
    if check_container_health "$container"; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
        log ERROR "健康检查失败，中止验收"
        echo ""
        log INFO "通过: ${passed}  失败: ${failed}"
        return 1
    fi

    # 阶段 2: 基础设施
    if check_infra; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
        log WARN "基础设施检查有告警（继续烟雾测试）"
    fi

    # 阶段 3: 烟雾测试
    if check_smoke_test "$container"; then
        passed=$((passed + 1))
    else
        failed=$((failed + 1))
    fi

    echo ""
    log INFO "========== 验收结果: 通过 ${passed} / 失败 ${failed} =========="

    if [ $failed -eq 0 ]; then
        log OK "所有验收检查通过！"
        return 0
    else
        log ERROR "存在 ${failed} 项失败"
        return 1
    fi
}

# ===================== Nginx 流量切换 =====================

switch_nginx() {
    local target_slot="$1"
    local container
    container=$(container_name "$target_slot")

    log STEP "切换 Nginx 流量 → ${container}"

    # 更新 upstream 配置
    echo "server ${container}:8080;" > "${NGINX_UPSTREAM}"

    # 重载 Nginx（不重启，零断连）
    if docker exec sub2api-nginx nginx -s reload 2>&1; then
        log OK "Nginx 已重载，流量指向 ${container}"
    else
        log ERROR "Nginx 重载失败"
        return 1
    fi
}

# ===================== 初始化 =====================

do_init() {
    log INFO "=========================================="
    log INFO "  Sub2API 蓝绿部署 - 首次初始化"
    log INFO "=========================================="

    cd "${DEPLOY_DIR}"

    # 检查必要文件
    if [ ! -f "${COMPOSE_FILE}" ]; then
        log ERROR "未找到 ${COMPOSE_FILE}"
        exit 1
    fi
    if [ ! -f ".env" ]; then
        log ERROR "未找到 .env 文件，请先运行 docker-deploy.sh"
        exit 1
    fi

    # 创建 nginx 目录，强制写入正确的 upstream（不管文件是否已存在）
    mkdir -p "${DEPLOY_DIR}/nginx"
    echo "server sub2api-blue:8080;" > "${NGINX_UPSTREAM}"
    log INFO "upstream.conf → sub2api-blue:8080"

    # 创建数据目录
    mkdir -p data postgres_data redis_data

    # 拉取镜像
    log STEP "拉取镜像..."
    docker compose -f "${COMPOSE_FILE}" pull

    # 启动基础设施 + blue 实例 + nginx
    log STEP "启动服务 (blue 实例)..."
    docker compose -f "${COMPOSE_FILE}" up -d nginx sub2api-blue postgres redis

    set_active_slot "blue"

    # 等待并验收
    sleep 5
    local container
    container=$(container_name "blue")

    if run_verification "$container"; then
        # 最终通过 Nginx 验证
        if check_nginx_routing; then
            echo ""
            log OK "=========================================="
            log OK "  初始化完成！蓝绿部署就绪"
            log OK "=========================================="
            echo ""
            log INFO "活跃实例: blue"
            log INFO "访问地址: http://$(curl -s --connect-timeout 3 ifconfig.me 2>/dev/null || echo 'YOUR_SERVER_IP'):${SERVER_PORT:-8080}"
            echo ""
        fi
    else
        log ERROR "初始化验收失败，请检查日志"
        log INFO "docker compose -f ${COMPOSE_FILE} logs sub2api-blue"
        return 1
    fi
}

# ===================== 蓝绿部署主流程 =====================

do_deploy() {
    log INFO "=========================================="
    log INFO "  Sub2API 零停机蓝绿部署"
    log INFO "=========================================="

    cd "${DEPLOY_DIR}"

    # 检查文件
    for f in "${COMPOSE_FILE}" ".env"; do
        if [ ! -f "$f" ]; then
            log ERROR "未找到 $f，请先运行 ./safe-deploy.sh --init"
            exit 1
        fi
    done

    local active
    active=$(get_active_slot)
    local standby
    standby=$(get_standby_slot)
    local active_container
    active_container=$(container_name "$active")
    local standby_container
    standby_container=$(container_name "$standby")

    log INFO "当前活跃: ${active_container}"
    log INFO "备用目标: ${standby_container}"

    # Step 1: 拉取新镜像
    log STEP "Step 1: 拉取最新镜像"
    local old_digest
    old_digest=$(docker inspect --format='{{.Id}}' "${IMAGE_NAME}:latest" 2>/dev/null || echo "none")

    docker pull "${IMAGE_NAME}:latest" > /dev/null 2>&1 || {
        log ERROR "镜像拉取失败，中止部署"
        exit 1
    }

    local new_digest
    new_digest=$(docker inspect --format='{{.Id}}' "${IMAGE_NAME}:latest" 2>/dev/null || echo "none")

    if [ "$old_digest" = "$new_digest" ] && [ "$old_digest" != "none" ]; then
        log INFO "镜像未更新 (digest 相同)，继续部署以确保配置生效"
    else
        log OK "新镜像已拉取"
    fi

    # Step 2: 启动备用实例（线上不受影响）
    log STEP "Step 2: 启动备用实例 ${standby_container}（线上不受影响）"

    # 清理可能残留的旧备用容器
    docker rm -f "${standby_container}" 2>/dev/null || true

    # 启动备用容器（green 需要 --profile green，blue 不需要 profile）
    if [ "$standby" = "green" ]; then
        docker compose -f "${COMPOSE_FILE}" --profile green up -d sub2api-green
    else
        docker compose -f "${COMPOSE_FILE}" up -d sub2api-blue
    fi

    if ! is_container_running "${standby_container}"; then
        log ERROR "${standby_container} 启动失败"
        return 1
    fi

    log OK "${standby_container} 已启动"

    # Step 3: 对备用实例做完整验收（此时线上流量仍在 active 上）
    log STEP "Step 3: 验收备用实例（线上流量不受影响）"
    sleep 3

    if run_verification "${standby_container}"; then
        # Step 4: 验收通过 → 切换流量
        log STEP "Step 4: 切换 Nginx 流量 ${active_container} → ${standby_container}"

        switch_nginx "${standby}"

        # 验证 Nginx 路由
        sleep 2
        if check_nginx_routing; then
            log OK "流量切换成功！"

            # Step 5: 等待旧连接排空后，停止旧实例
            log STEP "Step 5: 等待 ${GRACEFUL_WAIT}s 让旧连接排空..."
            sleep $GRACEFUL_WAIT

            docker stop "${active_container}" > /dev/null 2>&1 || true
            docker rm "${active_container}" > /dev/null 2>&1 || true
            log OK "旧实例 ${active_container} 已优雅下线"

            # 更新状态
            set_active_slot "${standby}"

            echo ""
            log OK "=========================================="
            log OK "  零停机部署成功！"
            log OK "=========================================="
            log INFO "活跃实例: ${standby_container}"
            log INFO "用户全程无感知"
            echo ""
            return 0
        else
            # Nginx 切换后路由异常 → 紧急回切
            log ERROR "Nginx 路由验证失败，紧急回切到 ${active_container}"
            switch_nginx "${active}"
            docker stop "${standby_container}" > /dev/null 2>&1 || true
            docker rm "${standby_container}" > /dev/null 2>&1 || true
            log WARN "已回切到 ${active_container}，线上恢复正常"
            return 1
        fi
    else
        # 验收失败 → 直接删掉备用实例，线上完全不受影响
        echo ""
        log ERROR "=========================================="
        log ERROR "  备用实例验收失败"
        log ERROR "  线上不受影响（流量仍在 ${active_container}）"
        log ERROR "=========================================="
        echo ""

        docker stop "${standby_container}" > /dev/null 2>&1 || true
        docker rm "${standby_container}" > /dev/null 2>&1 || true
        log INFO "已清理备用实例 ${standby_container}"
        log INFO "查看错误日志: docker logs ${standby_container} --tail 50"
        return 1
    fi
}

# ===================== 回滚 =====================

do_rollback() {
    local active
    active=$(get_active_slot)
    local standby
    standby=$(get_standby_slot)
    local active_container
    active_container=$(container_name "$active")
    local standby_container
    standby_container=$(container_name "$standby")

    log WARN "手动回滚: ${active_container} → ${standby_container}"

    cd "${DEPLOY_DIR}"

    # 启动旧实例
    docker rm -f "${standby_container}" 2>/dev/null || true
    if [ "$standby" = "green" ]; then
        docker compose -f "${COMPOSE_FILE}" --profile green up -d sub2api-green
    else
        docker compose -f "${COMPOSE_FILE}" up -d sub2api-blue
    fi

    sleep 5

    if check_container_health "${standby_container}"; then
        switch_nginx "${standby}"
        sleep 2

        if check_nginx_routing; then
            sleep $GRACEFUL_WAIT
            docker stop "${active_container}" > /dev/null 2>&1 || true
            docker rm "${active_container}" > /dev/null 2>&1 || true
            set_active_slot "${standby}"
            log OK "回滚成功！活跃实例: ${standby_container}"
            return 0
        else
            log ERROR "回滚后 Nginx 路由异常，回切"
            switch_nginx "${active}"
            return 1
        fi
    else
        log ERROR "回滚实例启动失败"
        docker stop "${standby_container}" > /dev/null 2>&1 || true
        return 1
    fi
}

# ===================== 查看状态 =====================

show_status() {
    local active
    active=$(get_active_slot)
    local port="${SERVER_PORT:-8080}"

    echo ""
    echo -e "${CYAN}========== Sub2API 蓝绿部署状态 ==========${NC}"
    echo ""
    echo -e "  活跃 Slot: ${GREEN}${active}${NC}"
    echo ""

    # 所有容器状态
    local containers=("sub2api-blue" "sub2api-green" "sub2api-nginx" "sub2api-postgres" "sub2api-redis")
    for c in "${containers[@]}"; do
        local status
        local health
        status=$(docker inspect -f '{{.State.Status}}' "$c" 2>/dev/null || echo "not_found")
        health=$(docker inspect -f '{{.State.Health.Status}}' "$c" 2>/dev/null || echo "-")

        local marker=""
        if [ "$c" = "sub2api-${active}" ]; then
            marker=" ← ACTIVE"
        fi

        if [ "$status" = "running" ]; then
            echo -e "  ${GREEN}●${NC} ${c}: running (health: ${health})${marker}"
        elif [ "$status" = "not_found" ]; then
            echo -e "  ${YELLOW}○${NC} ${c}: not running${marker}"
        else
            echo -e "  ${RED}●${NC} ${c}: ${status}${marker}"
        fi
    done

    echo ""

    # Nginx 当前 upstream
    if [ -f "${NGINX_UPSTREAM}" ]; then
        echo -e "  Nginx upstream: $(cat "${NGINX_UPSTREAM}")"
    fi

    # 通过 Nginx 的健康检查
    local health_resp
    health_resp=$(curl -s --connect-timeout 3 --max-time 5 "http://localhost:${port}/health" 2>/dev/null || echo "unreachable")
    echo -e "  对外健康检查: ${health_resp}"

    echo ""
    echo -e "${CYAN}===========================================${NC}"
    echo ""
}

# ===================== 自动更新模式 =====================

do_auto_update() {
    cd "${DEPLOY_DIR}"

    if [ ! -f "${COMPOSE_FILE}" ] || [ ! -f ".env" ]; then
        exit 0
    fi

    local local_digest
    local_digest=$(docker inspect --format='{{.Id}}' "${IMAGE_NAME}:latest" 2>/dev/null || echo "none")

    docker pull "${IMAGE_NAME}:latest" > /dev/null 2>&1 || exit 0

    local remote_digest
    remote_digest=$(docker inspect --format='{{.Id}}' "${IMAGE_NAME}:latest" 2>/dev/null || echo "none")

    if [ "$local_digest" = "$remote_digest" ]; then
        exit 0
    fi

    log INFO "检测到新版本镜像，启动零停机部署..."
    do_deploy
}

# ===================== Cron 管理 =====================

setup_cron() {
    local interval="${1:-5}"
    local cron_cmd="*/${interval} * * * * ${DEPLOY_DIR}/safe-deploy.sh --auto >> ${LOG_FILE} 2>&1"
    local cron_marker="# sub2api-auto-deploy"

    crontab -l 2>/dev/null | grep -v "${cron_marker}" | crontab - 2>/dev/null || true
    (crontab -l 2>/dev/null; echo "${cron_cmd} ${cron_marker}") | crontab -

    log OK "自动更新已启用: 每 ${interval} 分钟检查"
    log INFO "关闭: $0 --cron-off"
}

remove_cron() {
    crontab -l 2>/dev/null | grep -v "sub2api-auto-deploy" | crontab - 2>/dev/null || true
    log OK "自动更新已关闭"
}

# ===================== 主入口 =====================

main() {
    case "${1:-}" in
        --init|-i)
            do_init
            ;;
        --auto|-a)
            do_auto_update
            ;;
        --rollback|-r)
            do_rollback
            ;;
        --status|-s)
            show_status
            ;;
        --logs|-l)
            if [ -f "${LOG_FILE}" ]; then
                tail -80 "${LOG_FILE}"
            else
                echo "暂无部署日志"
            fi
            ;;
        --verify|-v)
            local active
            active=$(get_active_slot)
            run_verification "$(container_name "$active")"
            ;;
        --cron)
            setup_cron "${2:-5}"
            ;;
        --cron-off)
            remove_cron
            ;;
        --help|-h)
            echo ""
            echo "Sub2API 零停机安全部署"
            echo ""
            echo "用法: $0 [选项]"
            echo ""
            echo "部署:"
            echo "  --init              首次初始化（启动全部基础设施 + blue 实例）"
            echo "  (无参数)            零停机蓝绿部署"
            echo "  --auto              自动模式：仅在有新镜像时部署（用于 cron）"
            echo "  --cron [分钟]       启用自动更新（默认每 5 分钟检查）"
            echo "  --cron-off          关闭自动更新"
            echo ""
            echo "运维:"
            echo "  --rollback          回滚到上一实例"
            echo "  --status            查看蓝绿部署状态"
            echo "  --verify            对当前活跃实例做验收检查"
            echo "  --logs              查看部署日志"
            echo ""
            echo "首次部署流程:"
            echo "  1. curl ... | bash          # 下载 compose + 生成密钥"
            echo "  2. ./safe-deploy.sh --init  # 初始化蓝绿部署"
            echo "  3. ./safe-deploy.sh --cron  # 开启自动更新"
            echo ""
            echo "更新部署流程（自动，无需手动操作）:"
            echo "  代码 push → CI 构建镜像 → cron 检测 → 蓝绿切换 → 用户无感知"
            echo ""
            ;;
        *)
            do_deploy
            ;;
    esac
}

main "$@"
