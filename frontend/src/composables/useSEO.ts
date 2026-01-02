import { useHead } from '@unhead/vue'
import { useI18n } from 'vue-i18n'

export interface SEOConfig {
  title?: string
  description?: string
  keywords?: string
  image?: string
  url?: string
  type?: string
  author?: string
  publishedTime?: string
  modifiedTime?: string
}

export function useSEO(config: SEOConfig = {}) {
  const { locale } = useI18n()
  const baseUrl = 'https://ai-in.one'
  const siteName = 'Sub2API - AI API Gateway'
  const defaultImage = `${baseUrl}/logo.png`

  const {
    title = siteName,
    description = 'Claude Code API 中转服务 - 稳定、高效、按需付费的 AI API 接入方案',
    keywords = 'claude api, claude code, codex api, ai api, openai api, gemini api, api gateway',
    image = defaultImage,
    url = baseUrl,
    type = 'website',
    author = 'Sub2API',
    publishedTime,
    modifiedTime
  } = config

  const fullTitle = title === siteName ? title : `${title} - ${siteName}`
  const fullUrl = url.startsWith('http') ? url : `${baseUrl}${url}`

  useHead({
    title: fullTitle,
    meta: [
      // 基础 meta 标签
      { name: 'description', content: description },
      { name: 'keywords', content: keywords },
      { name: 'author', content: author },

      // Open Graph 标签
      { property: 'og:site_name', content: siteName },
      { property: 'og:title', content: fullTitle },
      { property: 'og:description', content: description },
      { property: 'og:type', content: type },
      { property: 'og:url', content: fullUrl },
      { property: 'og:image', content: image },
      { property: 'og:locale', content: locale.value === 'zh' ? 'zh_CN' : 'en_US' },

      // Twitter Card 标签
      { name: 'twitter:card', content: 'summary_large_image' },
      { name: 'twitter:title', content: fullTitle },
      { name: 'twitter:description', content: description },
      { name: 'twitter:image', content: image },

      // 文章类型特殊标签
      ...(publishedTime ? [{ property: 'article:published_time', content: publishedTime }] : []),
      ...(modifiedTime ? [{ property: 'article:modified_time', content: modifiedTime }] : []),
    ],
    link: [
      // Canonical URL
      { rel: 'canonical', href: fullUrl },

      // 语言版本
      { rel: 'alternate', hreflang: 'zh', href: `${baseUrl}/zh${url}` },
      { rel: 'alternate', hreflang: 'en', href: `${baseUrl}/en${url}` },
      { rel: 'alternate', hreflang: 'x-default', href: fullUrl },
    ]
  })
}

// 预定义的 SEO 配置
export const seoConfigs = {
  home: {
    title: 'Claude Code API 中转服务',
    description: '提供 Claude Code、Codex、OpenAI、Gemini API 中转服务，稳定高效，按需付费，无需繁琐的海外账号注册',
    keywords: 'claude api, claude code, codex api, ai api 中转, openai api, gemini api, api gateway, ai 接口',
    url: '/home'
  },
  pricing: {
    title: 'API 服务定价',
    description: '灵活的 Claude Code API 定价方案，支持按量付费，透明计费，多种套餐可选',
    keywords: 'claude api 价格, api 定价, claude code 价格, ai api 费用',
    url: '/pricing'
  },
  docs: {
    title: 'API 文档',
    description: '完整的 Claude Code API 接入文档，包含快速开始、API 参考、代码示例等',
    keywords: 'claude api 文档, api 接入教程, claude code 教程, api 使用指南',
    url: '/docs'
  },
  quickStart: {
    title: '快速开始 - API 文档',
    description: '5 分钟快速接入 Claude Code API，详细的接入步骤和代码示例',
    keywords: 'claude api 快速开始, 接入教程, 快速集成, api 教程',
    url: '/docs/quick-start'
  },
  apiReference: {
    title: 'API 参考文档',
    description: '完整的 Claude Code API 接口文档，包含所有端点、参数说明、模型列表和错误处理',
    keywords: 'claude api 参考, api 接口文档, 参数说明, 错误代码',
    url: '/docs/api-reference'
  },
  examples: {
    title: '代码示例 - API 文档',
    description: '丰富的 Claude Code API 代码示例，包含 Python、JavaScript、Go 等多种语言的实战代码',
    keywords: 'claude api 示例, 代码示例, python 示例, javascript 示例, api 实战',
    url: '/docs/examples'
  },
  features: {
    title: '功能特性',
    description: '了解 Sub2API 的核心功能：统一接口、灵活计费、并发控制、使用统计等',
    keywords: 'api gateway 功能, claude api 特性, 接口管理',
    url: '/features'
  },
  login: {
    title: '登录',
    description: '登录 Sub2API 账号，管理您的 API Keys 和使用情况',
    url: '/login'
  },
  register: {
    title: '注册',
    description: '注册 Sub2API 账号，立即获取免费试用额度',
    url: '/register'
  },
  dashboard: {
    title: '仪表板',
    description: '管理您的 API Keys、订阅和使用统计',
    url: '/dashboard'
  }
}
