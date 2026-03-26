import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BulkAccountTestModal from '../BulkAccountTestModal.vue'

const { getAvailableModels } = vi.hoisted(() => ({
  getAvailableModels: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.geminiImagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'admin.accounts.bulkTest.current' && params?.name) {
          return `Current: ${params.name}`
        }
        if (key === 'admin.accounts.geminiImageReceived' && params?.count) {
          return `received-${params.count}`
        }
        return messages[key] || key
      }
    })
  }
})

function createStreamResponse(lines: string[]) {
  const encoder = new TextEncoder()
  const chunks = lines.map((line) => encoder.encode(line))
  let index = 0

  return {
    ok: true,
    body: {
      getReader: () => ({
        read: vi.fn().mockImplementation(async () => {
          if (index < chunks.length) {
            return { done: false, value: chunks[index++] }
          }
          return { done: true, value: undefined }
        })
      })
    }
  } as Response
}

function createErrorResponse(status: number, body: string) {
  return {
    ok: false,
    status,
    text: vi.fn().mockResolvedValue(body)
  } as unknown as Response
}

function mountModal(accounts: any[]) {
  return mount(BulkAccountTestModal, {
    props: {
      show: false,
      accounts
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: { template: '<div class="select-stub"></div>' },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true
      }
    }
  })
}

describe('BulkAccountTestModal', () => {
  beforeEach(() => {
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('同平台账号会使用模型交集并在部分失败时发出重试 ID', async () => {
    getAvailableModels
      .mockResolvedValueOnce([
        { id: 'gpt-5.4', display_name: 'GPT-5.4' },
        { id: 'gpt-5.1', display_name: 'GPT-5.1' }
      ])
      .mockResolvedValueOnce([
        { id: 'gpt-5.4', display_name: 'GPT-5.4' }
      ])

    global.fetch = vi.fn()
      .mockResolvedValueOnce(createStreamResponse([
        'data: {"type":"test_start","model":"gpt-5.4"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ]))
      .mockResolvedValueOnce(createErrorResponse(401, '{"error":{"message":"deactivated"}}')) as any

    const wrapper = mountModal([
      { id: 1, name: 'OpenAI-1', platform: 'openai', type: 'oauth', status: 'active' },
      { id: 2, name: 'OpenAI-2', platform: 'openai', type: 'oauth', status: 'active' }
    ])

    await wrapper.setProps({ show: true })
    await flushPromises()

    const startButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(getAvailableModels).toHaveBeenCalledTimes(2)
    expect(global.fetch).toHaveBeenCalledTimes(2)
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body)).toEqual({ model_id: 'gpt-5.4' })
    expect(JSON.parse((global.fetch as any).mock.calls[1][1].body)).toEqual({ model_id: 'gpt-5.4' })

    const completed = wrapper.emitted('completed')
    expect(completed).toBeTruthy()
    expect(completed?.[0]?.[0]).toEqual({
      failedIds: [2],
      skippedIds: [],
      retryIds: [2]
    })
  })

  it('停止后续时会在当前账号结束后跳过剩余账号', async () => {
    getAvailableModels.mockReset()
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])

    let resolveFirstFetch: ((value: Response) => void) | null = null
    global.fetch = vi.fn()
      .mockImplementationOnce(() => new Promise<Response>((resolve) => {
        resolveFirstFetch = resolve
      }))
      .mockResolvedValueOnce(createStreamResponse([
        'data: {"type":"test_start","model":"gpt-5.4"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])) as any

    const wrapper = mountModal([
      { id: 1, name: 'OpenAI-1', platform: 'openai', type: 'oauth', status: 'active' },
      { id: 2, name: 'OpenAI-2', platform: 'openai', type: 'oauth', status: 'active' }
    ])

    await wrapper.setProps({ show: true })
    await flushPromises()

    const startButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()
    await startButton!.trigger('click')
    await flushPromises()

    const stopButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.bulkTest.stopAfterCurrent'))
    expect(stopButton).toBeTruthy()
    await stopButton!.trigger('click')

    resolveFirstFetch?.(createStreamResponse([
      'data: {"type":"test_start","model":"gpt-5.4"}\n',
      'data: {"type":"test_complete","success":true}\n'
    ]))

    await flushPromises()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const completed = wrapper.emitted('completed')
    expect(completed?.[0]?.[0]).toEqual({
      failedIds: [],
      skippedIds: [2],
      retryIds: [2]
    })
  })
})
