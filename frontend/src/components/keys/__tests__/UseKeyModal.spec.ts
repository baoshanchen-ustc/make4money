import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

describe('UseKeyModal', () => {
  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  describe('Anthropic env snippets', () => {
    function mountAnthropic() {
      return mount(UseKeyModal, {
        props: {
          show: true,
          apiKey: 'sk-test',
          baseUrl: 'https://example.com/v1',
          platform: 'anthropic'
        },
        global: {
          stubs: {
            BaseDialog: {
              template: '<div><slot /><slot name="footer" /></div>'
            },
            Icon: {
              template: '<span />'
            }
          }
        }
      })
    }

    // Shell tabs use literal labels (not i18n keys): "macOS / Linux", "Windows CMD", "PowerShell".
    const shellTabLabels: Record<'unix' | 'cmd' | 'powershell', string> = {
      unix: 'macOS / Linux',
      cmd: 'Windows CMD',
      powershell: 'PowerShell'
    }

    async function clickShellTab(wrapper: ReturnType<typeof mountAnthropic>, tabId: 'unix' | 'cmd' | 'powershell') {
      const label = shellTabLabels[tabId]
      const tabButton = wrapper.findAll('button').find((button) => button.text().includes(label))
      expect(tabButton, `expected to find shell tab "${label}"`).toBeDefined()
      await tabButton!.trigger('click')
      await nextTick()
    }

    function shellSnippetText(wrapper: ReturnType<typeof mountAnthropic>) {
      // Anthropic platform renders two FileConfig blocks: shell env, then settings.json.
      // First <pre><code> block is the shell snippet.
      const blocks = wrapper.findAll('pre code')
      expect(blocks.length, 'expected at least 2 code blocks (shell + settings.json)').toBeGreaterThanOrEqual(2)
      return blocks[0].text()
    }

    function settingsJsonText(wrapper: ReturnType<typeof mountAnthropic>) {
      const blocks = wrapper.findAll('pre code')
      expect(blocks.length).toBeGreaterThanOrEqual(2)
      return blocks[1].text()
    }

    it.each(['unix', 'cmd', 'powershell'] as const)(
      'shell snippet (%s) includes the four required Claude Code env vars',
      async (tabId) => {
        const wrapper = mountAnthropic()
        await clickShellTab(wrapper, tabId)

        const text = shellSnippetText(wrapper)
        expect(text).toContain('ANTHROPIC_BASE_URL')
        expect(text).toContain('ANTHROPIC_AUTH_TOKEN')
        expect(text).toContain('CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1')
        expect(text).toContain('CLAUDE_CODE_ATTRIBUTION_HEADER=0')
      }
    )

    it('settings.json block keeps all four env vars', async () => {
      const wrapper = mountAnthropic()
      await clickShellTab(wrapper, 'unix')

      const text = settingsJsonText(wrapper)
      expect(text).toContain('"ANTHROPIC_BASE_URL"')
      expect(text).toContain('"ANTHROPIC_AUTH_TOKEN"')
      expect(text).toContain('"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"')
      expect(text).toContain('"CLAUDE_CODE_ATTRIBUTION_HEADER": "0"')
    })

    it('default snippets do NOT include 3P OTEL or DISABLE_TELEMETRY', async () => {
      const wrapper = mountAnthropic()
      await clickShellTab(wrapper, 'unix')

      const shell = shellSnippetText(wrapper)
      const settings = settingsJsonText(wrapper)
      for (const env of ['CLAUDE_CODE_ENABLE_TELEMETRY', 'OTEL_EXPORTER', 'OTEL_SERVICE_NAME', 'DISABLE_TELEMETRY']) {
        expect(shell, `shell snippet should not include ${env}`).not.toContain(env)
        expect(settings, `settings.json should not include ${env}`).not.toContain(env)
      }
    })
  })
})
