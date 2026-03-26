export interface AccountTestStreamEvent {
  type: string
  text?: string
  model?: string
  success?: boolean
  error?: string
  image_url?: string
  mime_type?: string
  status?: string
  code?: string
  data?: unknown
}

export interface StreamAccountTestOptions {
  modelId?: string
  prompt?: string
  isSora?: boolean
  authToken?: string | null
  signal?: AbortSignal
  fetchImpl?: typeof fetch
  onEvent?: (event: AccountTestStreamEvent) => void | Promise<void>
}

const sseDataPrefix = /^data:\s*/

const resolveAuthToken = (authToken?: string | null) => {
  if (typeof authToken === 'string') return authToken
  if (typeof localStorage === 'undefined') return ''
  return localStorage.getItem('auth_token') || ''
}

const buildTestBody = (options: StreamAccountTestOptions) => {
  if (options.isSora) return {}

  const body: Record<string, string> = {}
  const modelId = options.modelId?.trim()
  const prompt = options.prompt?.trim()
  if (modelId) {
    body.model_id = modelId
  }
  if (prompt) {
    body.prompt = prompt
  }
  return body
}

const extractHTTPErrorMessage = (status: number, rawBody: string) => {
  const body = rawBody.trim()
  if (!body) {
    return `HTTP error! status: ${status}`
  }

  try {
    const parsed = JSON.parse(body)
    const message = parsed?.message || parsed?.error?.message || parsed?.error
    if (typeof message === 'string' && message.trim()) {
      return message.trim()
    }
  } catch {
    // Fall through to plain-text body handling.
  }

  return body
}

const emitSSEEvents = async (chunk: string, onEvent?: (event: AccountTestStreamEvent) => void | Promise<void>) => {
  const lines = chunk.split('\n')
  const remainder = lines.pop() || ''

  for (const line of lines) {
    if (!sseDataPrefix.test(line)) continue

    const jsonStr = line.replace(sseDataPrefix, '').trim()
    if (!jsonStr) continue

    try {
      const event = JSON.parse(jsonStr) as AccountTestStreamEvent
      if (onEvent) {
        await onEvent(event)
      }
    } catch (error) {
      console.error('Failed to parse account test SSE event:', error)
    }
  }

  return remainder
}

export async function streamAccountTest(accountId: number, options: StreamAccountTestOptions = {}) {
  const fetchImpl = options.fetchImpl || fetch
  const authToken = resolveAuthToken(options.authToken)
  const response = await fetchImpl(`/api/v1/admin/accounts/${accountId}/test`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${authToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(buildTestBody(options)),
    signal: options.signal
  })

  if (!response.ok) {
    const bodyText = await response.text()
    throw new Error(extractHTTPErrorMessage(response.status, bodyText))
  }

  const reader = response.body?.getReader()
  if (!reader) {
    throw new Error('No response body')
  }

  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    buffer = await emitSSEEvents(buffer, options.onEvent)
  }

  buffer += decoder.decode()
  await emitSSEEvents(buffer, options.onEvent)
}
