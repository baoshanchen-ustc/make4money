import { sanitizeHtml } from './sanitize'

let markedModulePromise: Promise<typeof import('marked')> | null = null

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

async function getMarkedModule(): Promise<typeof import('marked')> {
  if (!markedModulePromise) {
    markedModulePromise = import('marked').then((module) => {
      module.marked.setOptions({
        breaks: true,
        gfm: true,
      })
      return module
    })
  }
  return markedModulePromise
}

export async function renderMarkdownToSafeHtml(content: string): Promise<string> {
  if (!content) return ''

  try {
    const { marked } = await getMarkedModule()
    return sanitizeHtml(marked.parse(content) as string)
  } catch (error) {
    console.error('Failed to render markdown, falling back to escaped text:', error)
    return escapeHtml(content).replace(/\n/g, '<br>')
  }
}
