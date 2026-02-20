#!/bin/bash
#
# Yi-Code 零停机部署脚本
# 先上传到临时位置，再原子替换，停机时间仅几秒
#

set -e

# ============================================================
# 配置区域
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 默认配置
REMOTE_HOST="YOUR_SERVER_IP"
REMOTE_USER="root"
REMOTE_DIR="/www/wwwroot/your-domain"
BINARY_NAME="code80"
SSH_KEY=""

# 加载本地配置
if [ -f "$SCRIPT_DIR/deploy.local.conf" ]; then
    source "$SCRIPT_DIR/deploy.local.conf"
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; sync_output; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; sync_output; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; sync_output; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; sync_output; }

# 强制刷新输出缓冲区
sync_output() {
    sleep 0.1
}

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        print_error "未找到命令: $1"
        exit 1
    fi
}

# SSH/SCP 命令封装
ssh_cmd() {
    local -a ssh_opts=(
        -o ConnectTimeout=10
        -o ServerAliveInterval=15
        -o ServerAliveCountMax=6
    )

    if [ -n "$SSH_KEY" ]; then
        ssh "${ssh_opts[@]}" -i "$SSH_KEY" "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    else
        ssh "${ssh_opts[@]}" "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    fi
}

run_scp() {
    local use_legacy="$1"
    shift

    local -a scp_opts=(
        -o ConnectTimeout=10
        -o ServerAliveInterval=15
        -o ServerAliveCountMax=6
    )

    if [ "$use_legacy" = true ]; then
        scp_opts=(-O "${scp_opts[@]}")
    fi

    if [ -n "$SSH_KEY" ]; then
        scp "${scp_opts[@]}" -i "$SSH_KEY" "$@"
    else
        scp "${scp_opts[@]}" "$@"
    fi
}

scp_cmd() {
    local max_attempts=3
    local attempt=1
    local legacy_mode=false

    while [ "$attempt" -le "$max_attempts" ]; do
        if [ "$attempt" -ge 2 ]; then
            legacy_mode=true
        fi

        if run_scp "$legacy_mode" "$@"; then
            return 0
        fi

        if [ "$legacy_mode" = false ]; then
            print_warning "SCP 默认模式失败，下一次将使用 -O 兼容模式重试。"
        else
            print_warning "上传失败，准备第 ${attempt}/${max_attempts} 次重试..."
        fi

        attempt=$((attempt + 1))
        sleep 2
    done

    return 1
}

ensure_local_binary_exists() {
    if [ ! -f "backend/$BINARY_NAME" ]; then
        print_error "未找到后端二进制: backend/$BINARY_NAME"
        print_info "请先执行完整部署，或确认 --skip-build 时该文件已存在。"
        exit 1
    fi
}

# ============================================================
# 主流程
# ============================================================

main() {
    echo ""
    echo "=============================================="
    echo "   Yi-Code 零停机部署 (原子替换)"
    echo "=============================================="
    echo ""

    # 解析参数
    SKIP_FRONTEND=false
    SKIP_BUILD=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-frontend)
                SKIP_FRONTEND=true
                shift
                ;;
            --skip-build)
                SKIP_BUILD=true
                shift
                ;;
            --help|-h)
                echo "用法: $0 [选项]"
                echo ""
                echo "选项:"
                echo "  --skip-frontend  跳过前端编译"
                echo "  --skip-build     跳过所有编译，只部署"
                echo "  -h, --help       显示帮助"
                exit 0
                ;;
            *)
                print_error "未知参数: $1"
                exit 1
                ;;
        esac
    done

    cd "$PROJECT_ROOT"
    print_info "项目目录: $PROJECT_ROOT"
    print_info "目标服务器: ${REMOTE_USER}@${REMOTE_HOST}"
    print_info "部署目录: ${REMOTE_DIR}"
    echo ""

    require_cmd ssh
    require_cmd scp

    # Step 1: 编译前端
    if [ "$SKIP_BUILD" = false ] && [ "$SKIP_FRONTEND" = false ]; then
        require_cmd pnpm
        print_info "Step 1/5: 编译前端..."
        cd frontend
        pnpm install --frozen-lockfile 2>/dev/null || pnpm install
        pnpm run build
        cd "$PROJECT_ROOT"
        print_success "前端编译完成"
    else
        print_warning "Step 1/5: 跳过前端编译"
    fi

    # Step 2: 编译后端 (交叉编译 linux/amd64)
    if [ "$SKIP_BUILD" = false ]; then
        require_cmd go
        print_info "Step 2/5: 编译后端 (linux/amd64)..."
        cd backend
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -tags embed -o "$BINARY_NAME" ./cmd/server
        cd "$PROJECT_ROOT"
        print_success "后端编译完成: backend/$BINARY_NAME"
    else
        print_warning "Step 2/5: 跳过后端编译"
    fi

    # Step 3: 上传到临时位置（服务继续运行）
    print_info "Step 3/5: 上传到临时位置（服务保持运行）..."
    TEMP_FILE="/tmp/${BINARY_NAME}.new.$$"
    ensure_local_binary_exists

    if ! scp_cmd "backend/$BINARY_NAME" "${REMOTE_USER}@${REMOTE_HOST}:${TEMP_FILE}"; then
        print_error "上传失败：连接中断或 SCP 协议不兼容。"
        print_info "可手动测试：scp -O backend/$BINARY_NAME ${REMOTE_USER}@${REMOTE_HOST}:${TEMP_FILE}"
        exit 1
    fi
    ssh_cmd "chmod +x ${TEMP_FILE}"

    print_success "上传完成: ${TEMP_FILE}"

    # Step 4: 原子替换（停机时间仅几秒）
    print_info "Step 4/5: 执行原子替换（停机约3-5秒）..."

    # 备份当前版本
    ssh_cmd "cd ${REMOTE_DIR}/backend && [ -f ${BINARY_NAME} ] && cp ${BINARY_NAME} ${BINARY_NAME}.backup" || true

    # 停止服务
    print_info "停止服务..."
    ssh_cmd "supervisorctl stop ${BINARY_NAME} || pkill -f ${BINARY_NAME} || true" 2>&1 | cat
    sleep 1
    print_info "服务已停止"

    # 原子替换
    print_info "执行文件替换..."
    ssh_cmd "mv ${TEMP_FILE} ${REMOTE_DIR}/backend/${BINARY_NAME}" 2>&1 | cat
    print_success "文件替换完成"

    # 启动服务
    print_info "启动服务..."
    ssh_cmd "cd ${REMOTE_DIR}/backend && (supervisorctl start ${BINARY_NAME} || (nohup ./${BINARY_NAME} > /tmp/${BINARY_NAME}.log 2>&1 & echo 'Service started in background'))" 2>&1 | cat

    sleep 2
    print_success "原子替换完成"

    # Step 5: 验证服务状态
    print_info "Step 5/5: 验证服务状态..."
    sleep 2

    if ssh_cmd "pgrep -f ${BINARY_NAME} > /dev/null"; then
        print_success "服务运行正常"

        # 显示进程信息
        echo ""
        ssh_cmd "ps aux | grep ${BINARY_NAME} | grep -v grep | head -1" 2>&1 | cat
        echo ""
    else
        print_error "服务启动失败！"
        print_info "查看日志..."
        ssh_cmd "tail -30 /tmp/${BINARY_NAME}.log" 2>&1 | cat || true

        # 尝试回滚
        print_warning "尝试回滚到备份版本..."
        ssh_cmd "cd ${REMOTE_DIR}/backend && [ -f ${BINARY_NAME}.backup ] && mv ${BINARY_NAME}.backup ${BINARY_NAME} && supervisorctl start ${BINARY_NAME}" 2>&1 | cat || true
        exit 1
    fi

    echo ""
    echo "=============================================="
    print_success "部署完成！停机时间约 3-5 秒"
    echo "=============================================="
    echo ""
    echo "访问地址: https://code.ai80.vip"
    echo ""
}

main "$@"
