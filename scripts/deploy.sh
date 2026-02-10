#!/bin/bash
#
# Yi-Code 自动化部署脚本
# 本地编译构建并部署到远程服务器
#

set -e

# ============================================================
# 配置区域 - 请根据你的服务器信息修改
# 或者创建 scripts/deploy.local.conf 文件覆盖默认配置
# ============================================================
REMOTE_HOST="YOUR_SERVER_IP"           # 服务器 IP 地址
REMOTE_USER="root"
REMOTE_DIR="/www/wwwroot/your-domain"  # 部署目录
BINARY_NAME="code80"
SSH_KEY=""  # 可选：指定 SSH 私钥路径，如 ~/.ssh/id_rsa

# 本地项目路径（脚本会自动检测）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 加载本地配置（如果存在）
if [ -f "$SCRIPT_DIR/deploy.local.conf" ]; then
    source "$SCRIPT_DIR/deploy.local.conf"
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# SSH 命令封装
ssh_cmd() {
    if [ -n "$SSH_KEY" ]; then
        ssh -i "$SSH_KEY" "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    else
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    fi
}

scp_cmd() {
    if [ -n "$SSH_KEY" ]; then
        scp -i "$SSH_KEY" "$@"
    else
        scp "$@"
    fi
}

# ============================================================
# 主流程
# ============================================================

main() {
    echo ""
    echo "=============================================="
    echo "       Yi-Code 自动化部署"
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

    # Step 1: 编译前端
    if [ "$SKIP_BUILD" = false ] && [ "$SKIP_FRONTEND" = false ]; then
        print_info "Step 1/4: 编译前端..."
        cd frontend
        pnpm install --frozen-lockfile 2>/dev/null || pnpm install
        pnpm run build
        cd "$PROJECT_ROOT"
        print_success "前端编译完成"
    else
        print_warning "Step 1/4: 跳过前端编译"
    fi

    # Step 2: 编译后端
    if [ "$SKIP_BUILD" = false ]; then
        print_info "Step 2/4: 编译后端 (linux/amd64)..."
        cd backend
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -tags embed -o "$BINARY_NAME" ./cmd/server
        cd "$PROJECT_ROOT"
        print_success "后端编译完成: backend/$BINARY_NAME"
    else
        print_warning "Step 2/4: 跳过后端编译"
    fi

    # Step 3: 上传到服务器（先传到临时位置，避免覆盖运行中的二进制）
    print_info "Step 3/4: 上传到服务器..."
    TEMP_FILE="/tmp/${BINARY_NAME}.new.$$"

    scp_cmd "backend/$BINARY_NAME" "${REMOTE_USER}@${REMOTE_HOST}:${TEMP_FILE}"
    ssh_cmd "chmod +x ${TEMP_FILE}"

    # 备份旧版本
    ssh_cmd "[ -f ${REMOTE_DIR}/backend/${BINARY_NAME} ] && cp ${REMOTE_DIR}/backend/${BINARY_NAME} ${REMOTE_DIR}/backend/${BINARY_NAME}.backup" || true

    print_success "上传完成"

    # Step 4: 停止服务、替换、重启
    print_info "Step 4/4: 停止服务并替换..."

    # 停止服务并等待进程完全退出
    ssh_cmd "supervisorctl stop $BINARY_NAME 2>/dev/null || pkill -f $BINARY_NAME || true"
    sleep 2
    # 确保进程已退出
    ssh_cmd "pkill -9 -f $BINARY_NAME 2>/dev/null || true"
    sleep 1

    # mv 原子替换（不受 Text file busy 影响）
    ssh_cmd "mv ${TEMP_FILE} ${REMOTE_DIR}/backend/${BINARY_NAME}"

    # 启动服务
    print_info "启动服务..."
    ssh_cmd "cd ${REMOTE_DIR}/backend && (supervisorctl start $BINARY_NAME 2>/dev/null || nohup ./$BINARY_NAME > /tmp/${BINARY_NAME}.log 2>&1 &)"

    sleep 2

    # 检查服务状态
    if ssh_cmd "pgrep -f $BINARY_NAME > /dev/null"; then
        print_success "服务启动成功"
    else
        print_error "服务启动失败，请检查日志"
        ssh_cmd "tail -20 /tmp/${BINARY_NAME}.log" || true

        # 尝试回滚
        print_warning "尝试回滚到备份版本..."
        ssh_cmd "cd ${REMOTE_DIR}/backend && [ -f ${BINARY_NAME}.backup ] && mv ${BINARY_NAME}.backup ${BINARY_NAME} && (supervisorctl start $BINARY_NAME 2>/dev/null || nohup ./$BINARY_NAME > /tmp/${BINARY_NAME}.log 2>&1 &)" || true
        exit 1
    fi

    echo ""
    echo "=============================================="
    print_success "部署完成！"
    echo "=============================================="
    echo ""
    echo "访问地址: https://code.ai80.vip"
    echo ""
}

main "$@"
