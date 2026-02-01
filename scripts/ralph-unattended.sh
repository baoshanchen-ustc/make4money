#!/bin/bash
# ============================================================================
# BMAD 无人值守自动化脚本
# 用 claude -p 非交互模式 + while 循环，上下文耗尽自动重启
# ============================================================================

set -uo pipefail

PROJECT_ROOT="/Volumes/SSD/ssd-code/github/yi-code"
SPRINT_STATUS="${PROJECT_ROOT}/_bmad-output/implementation-artifacts/sprint-status.yaml"
LOG_DIR="${PROJECT_ROOT}/logs/ralph"
MAX_ROUNDS=50  # 最多重启次数，防止无限循环

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

mkdir -p "${LOG_DIR}"

# 检查是否还有未完成的 Story
has_remaining_stories() {
    grep -qE '(ready-for-dev|in-progress)' "${SPRINT_STATUS}" 2>/dev/null
}

# 统计进度
show_progress() {
    local done_count pending_count
    done_count=$(grep -cE '^  [0-9]+-[0-9]+-.*: done' "${SPRINT_STATUS}" 2>/dev/null || echo 0)
    pending_count=$(grep -cE '^  [0-9]+-[0-9]+-.*: (ready-for-dev|in-progress|review)' "${SPRINT_STATUS}" 2>/dev/null || echo 0)
    local total=$((done_count + pending_count))
    echo -e "${CYAN}[进度]${NC} 完成: ${GREEN}${done_count}${NC}/${total} | 剩余: ${YELLOW}${pending_count}${NC}"
}

# 构建给 Claude 的提示词
build_prompt() {
    cat << 'PROMPT'
你是一个 BMAD 自动化编排引擎，负责按 Sprint 顺序完成剩余用户故事。

## 执行流程

1. 读取 `_bmad-output/implementation-artifacts/sprint-status.yaml`
2. 找到第一个状态为 `ready-for-dev` 或 `in-progress` 的 Story
3. 对该 Story 执行完整生命周期：

### 实现（dev-story 工作流）
- 读取 `_bmad/core/tasks/workflow.xml` 完整内容
- 使用 `_bmad/bmm/workflows/4-implementation/dev-story/workflow.yaml` 作为 workflow-config
- 严格按照 workflow.xml 指令执行
- 确保测试通过（`cd backend && make test-unit`）和编译成功（`cd backend && go build ./...`）
- 如有前端改动：`cd frontend && pnpm run test:run`
- 更新 Story Status 为 `review`，更新 sprint-status.yaml

### 代码审查（code-review 工作流）
- 读取 `_bmad/core/tasks/workflow.xml` 完整内容
- 使用 `_bmad/bmm/workflows/4-implementation/code-review/workflow.yaml` 作为 workflow-config
- 执行对抗性代码审查

### 自动修复
- 对所有 HIGH 和 MEDIUM 问题自动修复
- 每次修复后重新运行测试
- 最多 3 轮

### 标记完成
- 更新 Story Status 为 `done`
- 更新 sprint-status.yaml
- 检查 Sprint 是否全部完成，是则更新 Sprint 状态为 `done`
- Git commit: `feat(recharge): complete story {story-key}`

4. 完成后，如果上下文还有空间，继续处理下一个 Story（重复步骤 1-3）
5. 如果上下文即将耗尽，输出当前进度并正常退出

## 错误处理
- 编译/测试失败：最多重试 3 次
- Story 完全失败：记录原因，回滚到 ready-for-dev，跳过继续下一个
- 绝不跳过测试和 CR

## 关键约束
- 每个 Story 一个 git commit
- 遵循现有代码风格
- 只修改当前 Story 相关的文件
PROMPT
}

# ========================== 主循环 ==========================

echo -e "\n${CYAN}============================================${NC}"
echo -e "${CYAN}  BMAD 无人值守自动化${NC}"
echo -e "${CYAN}============================================${NC}\n"

show_progress

round=0
while has_remaining_stories && [ $round -lt $MAX_ROUNDS ]; do
    round=$((round + 1))
    timestamp=$(date +%Y%m%d-%H%M%S)
    log_file="${LOG_DIR}/round-${round}-${timestamp}.log"

    echo -e "\n${BLUE}[第 ${round} 轮]${NC} $(date '+%H:%M:%S') 启动 Claude..."
    show_progress

    # 用 claude -p 非交互模式执行
    cd "${PROJECT_ROOT}"
    PROMPT=$(build_prompt)
    claude -p "${PROMPT}" --allowedTools 'Edit,Write,Bash,Read,Glob,Grep,Skill' 2>&1 | tee "${log_file}"

    exit_code=$?
    echo -e "\n${YELLOW}[第 ${round} 轮结束]${NC} 退出码: ${exit_code} | $(date '+%H:%M:%S')"
    show_progress

    # 检查是否还有剩余
    if ! has_remaining_stories; then
        echo -e "\n${GREEN}所有 Story 已完成!${NC}"
        break
    fi

    echo -e "${BLUE}还有未完成的 Story，5 秒后启动下一轮...${NC}"
    sleep 5
done

echo -e "\n${CYAN}============================================${NC}"
echo -e "${CYAN}  自动化结束${NC}"
echo -e "${CYAN}============================================${NC}"
echo -e "总轮次: ${round}"
show_progress
echo -e "日志目录: ${LOG_DIR}"
