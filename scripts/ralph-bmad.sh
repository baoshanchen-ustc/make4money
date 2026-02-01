#!/bin/bash
# ============================================================================
# Ralph Loop + BMAD 自动编排脚本
# 使用 tmux + caffeinate 在后台按 Sprint 顺序自动完成所有用户故事
# ============================================================================

set -euo pipefail

# ========================== 配置 ==========================
PROJECT_ROOT="/Volumes/SSD/ssd-code/github/yi-code"
SPRINT_STATUS="${PROJECT_ROOT}/_bmad-output/implementation-artifacts/sprint-status.yaml"
LOG_DIR="${PROJECT_ROOT}/logs/ralph"
SESSION_PREFIX="ralph-sprint"

# ========================== 颜色输出 ==========================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ========================== 辅助函数 ==========================

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_err()  { echo -e "${RED}[ERROR]${NC} $*"; }

usage() {
    cat << 'EOF'
Ralph Loop + BMAD 自动编排脚本

用法:
  ralph-bmad.sh <命令> [参数]

命令:
  start [sprint-N]    启动自动化（默认从当前 Sprint 开始，可指定 Sprint）
  start-all           启动所有剩余 Sprint 的自动化
  status              查看所有运行中的 Sprint 会话状态
  attach <sprint-N>   连接到指定 Sprint 的 tmux 会话
  logs <sprint-N>     查看指定 Sprint 的日志
  stop <sprint-N>     停止指定 Sprint 的会话
  stop-all            停止所有 Ralph Loop 会话
  progress            查看整体进度（解析 sprint-status.yaml）

示例:
  ralph-bmad.sh start              # 从当前进行中的 Sprint 开始
  ralph-bmad.sh start sprint-2     # 从 Sprint 2 开始
  ralph-bmad.sh start-all          # 按顺序完成所有 Sprint
  ralph-bmad.sh status             # 查看状态
  ralph-bmad.sh attach sprint-1    # 连接到 Sprint 1 会话
  ralph-bmad.sh progress           # 查看总体进度
EOF
}

# 确保日志目录存在
ensure_log_dir() {
    mkdir -p "${LOG_DIR}"
}

# 获取当前进行中的 Sprint
get_current_sprint() {
    grep -E '^\s+sprint-[0-9]+: in-progress' "${SPRINT_STATUS}" | head -1 | sed 's/.*\(sprint-[0-9]\+\).*/\1/' || echo ""
}

# 获取指定 Sprint 的待办 Story 列表
get_sprint_stories() {
    local sprint_num="$1"
    local in_sprint=false
    local stories=()

    while IFS= read -r line; do
        # 检测 Sprint 区域开始
        if echo "$line" | grep -qE "sprint-${sprint_num}:"; then
            in_sprint=true
            continue
        fi

        # 检测下一个 Sprint 区域（结束当前区域）
        if $in_sprint && echo "$line" | grep -qE "sprint-[0-9]+:"; then
            break
        fi

        # 提取 ready-for-dev 或 in-progress 的 story
        if $in_sprint; then
            local story_key
            story_key=$(echo "$line" | grep -oE '[0-9]+-[0-9]+-[a-z0-9-]+' | head -1)
            local status
            status=$(echo "$line" | grep -oE '(ready-for-dev|in-progress)' | head -1)

            if [ -n "$story_key" ] && [ -n "$status" ]; then
                # 跳过可选的 2-8
                if [ "$story_key" != "2-8-jsapi-pay-invoke" ]; then
                    stories+=("$story_key")
                fi
            fi
        fi
    done < "${SPRINT_STATUS}"

    echo "${stories[@]}"
}

# 获取所有未完成的 Sprint 编号
get_pending_sprints() {
    local sprints=()
    for i in 1 2 3 4 5 6; do
        local status
        status=$(grep -E "sprint-${i}:" "${SPRINT_STATUS}" | grep -oE '(backlog|in-progress)' | head -1)
        if [ -n "$status" ]; then
            sprints+=("$i")
        fi
    done
    echo "${sprints[@]}"
}

# 启动单个 Sprint 的 Ralph Loop
start_sprint() {
    local sprint_num="$1"
    local session_name="${SESSION_PREFIX}-${sprint_num}"
    local log_file="${LOG_DIR}/sprint-${sprint_num}-$(date +%Y%m%d-%H%M%S).log"

    ensure_log_dir

    # 检查会话是否已存在
    if tmux has-session -t "$session_name" 2>/dev/null; then
        log_warn "Sprint ${sprint_num} 的会话已在运行: ${session_name}"
        log_info "使用 'ralph-bmad.sh attach sprint-${sprint_num}' 连接查看"
        return 1
    fi

    # 获取待办 Story
    local stories
    stories=$(get_sprint_stories "$sprint_num")
    if [ -z "$stories" ]; then
        log_ok "Sprint ${sprint_num} 没有待办 Story（可能已全部完成）"
        return 0
    fi

    log_info "Sprint ${sprint_num} 待办 Story: ${stories}"
    log_info "日志文件: ${log_file}"

    # 创建 tmux 会话并启动 Ralph Loop
    tmux new-session -d -s "$session_name" -c "${PROJECT_ROOT}"

    # 在 tmux 会话中执行 Claude Code + Ralph Loop
    tmux send-keys -t "$session_name" "caffeinate -i claude 2>&1 | tee '${log_file}'" Enter

    # 等待 Claude 启动
    sleep 3

    # 发送 Ralph Loop 命令
    local ralph_prompt="/ralph-loop \"执行 /ralph-bmad 命令完成 Sprint ${sprint_num} 的所有剩余用户故事。

按照 /ralph-bmad 插件定义的工作流程：
1. 解析 sprint-status.yaml 获取 Sprint ${sprint_num} 的待办 Story
2. 按顺序执行每个 Story 的完整生命周期：
   - 调用 BMAD dev-story 工作流实现故事
   - 调用 BMAD code-review 工作流进行代码审查
   - 自动修复所有 HIGH/MEDIUM CR 问题
   - 标记完成并更新 sprint-status.yaml
   - 执行 /compact 压缩上下文
3. 完成后输出最终报告

重要约束：
- 严格按 Story 文件中的 AC 和 Tasks 实现
- 每个 Story 必须通过测试和编译
- 每个 Story 完成后 git commit
- 每个 Story 完成后 /compact 压缩上下文
\" --max-iterations 200 --completion-promise \"SPRINT_${sprint_num}_COMPLETE\""

    tmux send-keys -t "$session_name" "$ralph_prompt" Enter

    log_ok "Sprint ${sprint_num} 自动化已启动"
    log_info "会话名称: ${session_name}"
    log_info "连接查看: ralph-bmad.sh attach sprint-${sprint_num}"
    log_info "查看日志: ralph-bmad.sh logs sprint-${sprint_num}"
}

# 按顺序启动所有 Sprint
start_all_sprints() {
    local pending
    pending=$(get_pending_sprints)

    if [ -z "$pending" ]; then
        log_ok "所有 Sprint 已完成"
        return 0
    fi

    log_info "将按顺序启动以下 Sprint: ${pending}"
    log_warn "注意：Sprint 按顺序执行，一个完成后才开始下一个"

    # 只启动第一个待办 Sprint
    local first_sprint
    first_sprint=$(echo "$pending" | awk '{print $1}')
    start_sprint "$first_sprint"

    log_info "Sprint ${first_sprint} 完成后，请再次运行 'ralph-bmad.sh start-all' 启动下一个"
}

# 查看所有会话状态
show_status() {
    echo -e "\n${CYAN}=== Ralph Loop + BMAD 会话状态 ===${NC}\n"

    local found=false
    for i in 1 2 3 4 5 6; do
        local session_name="${SESSION_PREFIX}-${i}"
        if tmux has-session -t "$session_name" 2>/dev/null; then
            found=true
            echo -e "  Sprint ${i}: ${GREEN}运行中${NC} (会话: ${session_name})"
        fi
    done

    if ! $found; then
        echo -e "  ${YELLOW}没有运行中的会话${NC}"
    fi

    echo ""
}

# 连接到指定 Sprint 的会话
attach_sprint() {
    local sprint_id="$1"
    local sprint_num
    sprint_num=$(echo "$sprint_id" | sed 's/sprint-//')
    local session_name="${SESSION_PREFIX}-${sprint_num}"

    if ! tmux has-session -t "$session_name" 2>/dev/null; then
        log_err "会话 ${session_name} 不存在"
        log_info "使用 'ralph-bmad.sh status' 查看运行中的会话"
        return 1
    fi

    log_info "正在连接到 ${session_name}（按 Ctrl+B D 分离）"
    tmux attach -t "$session_name"
}

# 查看日志
show_logs() {
    local sprint_id="$1"
    local sprint_num
    sprint_num=$(echo "$sprint_id" | sed 's/sprint-//')
    local latest_log
    latest_log=$(ls -t "${LOG_DIR}"/sprint-"${sprint_num}"-*.log 2>/dev/null | head -1)

    if [ -z "$latest_log" ]; then
        log_err "找不到 Sprint ${sprint_num} 的日志"
        return 1
    fi

    log_info "查看日志: ${latest_log}"
    tail -f "$latest_log"
}

# 停止指定 Sprint
stop_sprint() {
    local sprint_id="$1"
    local sprint_num
    sprint_num=$(echo "$sprint_id" | sed 's/sprint-//')
    local session_name="${SESSION_PREFIX}-${sprint_num}"

    if tmux has-session -t "$session_name" 2>/dev/null; then
        tmux kill-session -t "$session_name"
        log_ok "已停止 Sprint ${sprint_num} 的会话"
    else
        log_warn "Sprint ${sprint_num} 的会话不存在"
    fi
}

# 停止所有会话
stop_all() {
    for i in 1 2 3 4 5 6; do
        local session_name="${SESSION_PREFIX}-${i}"
        if tmux has-session -t "$session_name" 2>/dev/null; then
            tmux kill-session -t "$session_name"
            log_ok "已停止 Sprint ${i}"
        fi
    done
    log_ok "所有会话已停止"
}

# 显示整体进度
show_progress() {
    echo -e "\n${CYAN}=====================================${NC}"
    echo -e "${CYAN}  BMAD Sprint 整体进度${NC}"
    echo -e "${CYAN}=====================================${NC}\n"

    local total_done=0
    local total_pending=0
    local total_in_progress=0

    for i in 1 2 3 4 5 6; do
        local sprint_status
        sprint_status=$(grep -E "^\s+sprint-${i}:" "${SPRINT_STATUS}" | grep -oE '(backlog|in-progress|done)' | head -1)

        local color="${YELLOW}"
        case "$sprint_status" in
            done) color="${GREEN}" ;;
            in-progress) color="${BLUE}" ;;
            backlog) color="${YELLOW}" ;;
        esac

        echo -e "  Sprint ${i}: ${color}${sprint_status}${NC}"

        # 统计 Story 状态
        local done_count=0
        local pending_count=0
        local ip_count=0
        local in_sprint=false

        while IFS= read -r line; do
            # 检测当前 Sprint 开始
            if echo "$line" | grep -qE "sprint-${i}:"; then
                in_sprint=true
                continue
            fi
            # 检测下一个 Sprint 或回顾（结束当前区域）
            if $in_sprint && echo "$line" | grep -qE "sprint-[0-9]+:|sprint-${i}-retrospective"; then
                break
            fi
            # 统计 Story
            if $in_sprint && echo "$line" | grep -qE '[0-9]+-[0-9]+-[a-z]'; then
                if echo "$line" | grep -q "done"; then
                    ((done_count++)) || true
                elif echo "$line" | grep -q "in-progress"; then
                    ((ip_count++)) || true
                elif echo "$line" | grep -q "ready-for-dev"; then
                    ((pending_count++)) || true
                fi
            fi
        done < "${SPRINT_STATUS}"

        total_done=$((total_done + done_count))
        total_pending=$((total_pending + pending_count))
        total_in_progress=$((total_in_progress + ip_count))

        if [ $((done_count + pending_count + ip_count)) -gt 0 ]; then
            echo -e "    完成: ${GREEN}${done_count}${NC} | 进行中: ${BLUE}${ip_count}${NC} | 待办: ${YELLOW}${pending_count}${NC}"
        fi
    done

    local total=$((total_done + total_pending + total_in_progress))
    local pct=0
    if [ $total -gt 0 ]; then
        pct=$((total_done * 100 / total))
    fi

    echo -e "\n${CYAN}-------------------------------------${NC}"
    echo -e "  总计: ${total} 个 Story"
    echo -e "  完成: ${GREEN}${total_done}${NC} (${pct}%)"
    echo -e "  进行中: ${BLUE}${total_in_progress}${NC}"
    echo -e "  待办: ${YELLOW}${total_pending}${NC}"
    echo -e "${CYAN}=====================================${NC}\n"
}

# ========================== 主入口 ==========================

main() {
    if [ $# -eq 0 ]; then
        usage
        exit 1
    fi

    local command="$1"
    shift

    case "$command" in
        start)
            if [ $# -gt 0 ]; then
                local sprint_num
                sprint_num=$(echo "$1" | sed 's/sprint-//')
                start_sprint "$sprint_num"
            else
                local current
                current=$(get_current_sprint)
                if [ -z "$current" ]; then
                    log_warn "没有进行中的 Sprint，查找第一个待办 Sprint..."
                    local pending
                    pending=$(get_pending_sprints)
                    if [ -z "$pending" ]; then
                        log_ok "所有 Sprint 已完成"
                        exit 0
                    fi
                    local first
                    first=$(echo "$pending" | awk '{print $1}')
                    start_sprint "$first"
                else
                    local sprint_num
                    sprint_num=$(echo "$current" | sed 's/sprint-//')
                    start_sprint "$sprint_num"
                fi
            fi
            ;;
        start-all)
            start_all_sprints
            ;;
        status)
            show_status
            ;;
        attach)
            if [ $# -eq 0 ]; then
                log_err "请指定 Sprint: ralph-bmad.sh attach sprint-1"
                exit 1
            fi
            attach_sprint "$1"
            ;;
        logs)
            if [ $# -eq 0 ]; then
                log_err "请指定 Sprint: ralph-bmad.sh logs sprint-1"
                exit 1
            fi
            show_logs "$1"
            ;;
        stop)
            if [ $# -eq 0 ]; then
                log_err "请指定 Sprint: ralph-bmad.sh stop sprint-1"
                exit 1
            fi
            stop_sprint "$1"
            ;;
        stop-all)
            stop_all
            ;;
        progress)
            show_progress
            ;;
        *)
            log_err "未知命令: ${command}"
            usage
            exit 1
            ;;
    esac
}

main "$@"
