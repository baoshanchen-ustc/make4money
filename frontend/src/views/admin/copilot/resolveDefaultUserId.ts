/**
 * 根据当前用户列表和已选 userId 决定下一个应选中的 userId。
 *
 * 规则：
 * - 列表为空 → null
 * - currentId 仍在列表中 → 保持不变
 * - 否则 → 优先选 Premium 最多的用户；若全部 Premium=0 则选列表第一个
 */
export function resolveDefaultUserId(
  users: { userId: number; premiumRequests: number }[],
  currentId: number | null,
): number | null {
  if (users.length === 0) return null
  const ids = new Set(users.map(u => u.userId))
  if (currentId !== null && ids.has(currentId)) return currentId
  const withPremium = users.filter(u => u.premiumRequests > 0)
  return withPremium.length > 0
    ? withPremium.reduce((a, b) => b.premiumRequests > a.premiumRequests ? b : a).userId
    : users[0].userId
}
