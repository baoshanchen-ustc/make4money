export const OPENAI_REQUEST_OVERRIDES_EXTRA_KEY = 'openai_request_overrides'
const DISALLOWED_TOP_LEVEL_KEYS = new Set(['model'])

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value)

export const stringifyOpenAIRequestOverrides = (value: unknown): string => {
  if (!isPlainObject(value) || Object.keys(value).length === 0) {
    return ''
  }

  return JSON.stringify(value, null, 2)
}

export const parseOpenAIRequestOverrides = (
  text: string
): { value: Record<string, unknown> | null; error: string | null } => {
  const trimmed = text.trim()
  if (!trimmed) {
    return { value: null, error: null }
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!isPlainObject(parsed)) {
      return { value: null, error: 'top_level_object_required' }
    }
    if (Object.keys(parsed).some((key) => DISALLOWED_TOP_LEVEL_KEYS.has(key.trim().toLowerCase()))) {
      return { value: null, error: 'model_not_allowed' }
    }
    return { value: parsed, error: null }
  } catch {
    return { value: null, error: 'invalid_json' }
  }
}
