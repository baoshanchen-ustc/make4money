// 分组「自定义图标」白名单（与后端 allowedDisplayIcons 同源）。
// 修改时请同步 backend/internal/handler/admin/group_handler.go 中的 allowedDisplayIcons。
export type DisplayIconKey =
  | 'anthropic'
  | 'claude'
  | 'claude-code'
  | 'openai'
  | 'chatgpt'
  | 'codex'
  | 'gemini'
  | 'gemini-cli'
  | 'antigravity'
  | 'bedrock'
  | 'deepseek'
  | 'generic'

export interface DisplayIconOption {
  key: DisplayIconKey
  label: string // 推荐展示名称（管理员可在「展示名称」字段中覆盖）
}

export const DISPLAY_ICON_OPTIONS: DisplayIconOption[] = [
  { key: 'anthropic', label: 'Anthropic' },
  { key: 'claude', label: 'Claude' },
  { key: 'claude-code', label: 'Claude Code' },
  { key: 'openai', label: 'OpenAI' },
  { key: 'chatgpt', label: 'ChatGPT' },
  { key: 'codex', label: 'Codex' },
  { key: 'gemini', label: 'Gemini' },
  { key: 'gemini-cli', label: 'Gemini CLI' },
  { key: 'antigravity', label: 'Antigravity' },
  { key: 'bedrock', label: 'AWS Bedrock' },
  { key: 'deepseek', label: 'DeepSeek' },
  { key: 'generic', label: '通用' }
]

export const DISPLAY_ICON_KEYS: readonly DisplayIconKey[] = DISPLAY_ICON_OPTIONS.map((o) => o.key)

export function isValidDisplayIcon(value: unknown): value is DisplayIconKey {
  return typeof value === 'string' && (DISPLAY_ICON_KEYS as readonly string[]).includes(value)
}

// ---- 颜色主题统一解析 ----
// 所有展示该分组的位置（badge 背景、文字颜色、倍率 pill）都应该按这里返回的 theme 着色，
// 而不是直接读 platform。否则即使 display_name + display_icon 已伪装为 Claude，
// 边框/背景仍会以 OpenAI 绿、Gemini 蓝等真实平台色泄露。
export type IconColorTheme = 'orange' | 'green' | 'blue' | 'violet' | 'cyan' | 'gray'

const iconKeyThemeMap: Record<DisplayIconKey, IconColorTheme> = {
  anthropic: 'orange',
  claude: 'orange',
  'claude-code': 'orange',
  openai: 'green',
  chatgpt: 'green',
  codex: 'green',
  gemini: 'blue',
  'gemini-cli': 'blue',
  antigravity: 'violet',
  bedrock: 'orange', // AWS 品牌色
  deepseek: 'cyan',
  generic: 'gray'
}

const platformFallbackTheme: Record<string, IconColorTheme> = {
  anthropic: 'orange',
  openai: 'green',
  gemini: 'blue',
  antigravity: 'violet',
  deepseek: 'cyan'
}

/**
 * resolveIconTheme 优先按 display_icon key 决定颜色家族；
 * 未配置或无效 key 时回退到 platform 默认色；都不命中则 gray。
 */
export function resolveIconTheme(
  iconKey?: string | null,
  platform?: string
): IconColorTheme {
  if (iconKey && iconKey in iconKeyThemeMap) {
    return iconKeyThemeMap[iconKey as DisplayIconKey]
  }
  if (platform && platform in platformFallbackTheme) {
    return platformFallbackTheme[platform]
  }
  return 'gray'
}
