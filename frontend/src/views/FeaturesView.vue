<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Header -->
    <header class="border-b border-gray-200 bg-white px-6 py-4 dark:border-dark-800 dark:bg-dark-900">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <router-link to="/home" class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img src="/logo.png" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-xl font-bold text-gray-900 dark:text-white">Sub2API</span>
        </router-link>

        <div class="flex items-center gap-4">
          <router-link to="/docs" class="text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white">
            文档
          </router-link>
          <router-link to="/pricing" class="text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white">
            定价
          </router-link>
          <router-link
            to="/login"
            class="rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
          >
            开始使用
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Hero Section -->
    <section class="px-6 py-16 lg:py-24">
      <div class="mx-auto max-w-6xl text-center">
        <h1 class="text-4xl font-bold tracking-tight text-gray-900 sm:text-5xl lg:text-6xl dark:text-white">
          强大的 <span class="bg-gradient-to-r from-primary-600 to-purple-600 bg-clip-text text-transparent">AI API</span> 中转服务
        </h1>
        <p class="mx-auto mt-6 max-w-2xl text-lg text-gray-600 dark:text-dark-300">
          为开发者提供稳定、高效、易用的 Claude、GPT、Gemini 等主流 AI 模型接入方案
        </p>
      </div>
    </section>

    <!-- Features Grid -->
    <section class="px-6 py-12">
      <div class="mx-auto max-w-6xl">
        <div class="grid gap-8 md:grid-cols-2 lg:grid-cols-3">
          <!-- Feature 1 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
              <svg class="h-6 w-6 text-primary-600 dark:text-primary-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">极速响应</h3>
            <p class="text-gray-600 dark:text-dark-300">
              全球 CDN 加速，智能路由选择，平均响应时间 &lt;100ms，为您的应用提供极致性能体验
            </p>
          </div>

          <!-- Feature 2 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-green-100 dark:bg-green-900/30">
              <svg class="h-6 w-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">99.9% 可用性</h3>
            <p class="text-gray-600 dark:text-dark-300">
              多节点容灾备份，自动故障转移，确保服务稳定运行，让您的业务永不掉线
            </p>
          </div>

          <!-- Feature 3 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-blue-100 dark:bg-blue-900/30">
              <svg class="h-6 w-6 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">企业级安全</h3>
            <p class="text-gray-600 dark:text-dark-300">
              TLS 1.3 加密传输，API Key 安全管理，完善的权限控制，保护您的数据安全
            </p>
          </div>

          <!-- Feature 4 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-purple-100 dark:bg-purple-900/30">
              <svg class="h-6 w-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">灵活定价</h3>
            <p class="text-gray-600 dark:text-dark-300">
              按需付费，无最低消费，支持包月套餐，企业定制方案，满足不同规模需求
            </p>
          </div>

          <!-- Feature 5 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-orange-100 dark:bg-orange-900/30">
              <svg class="h-6 w-6 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">实时监控</h3>
            <p class="text-gray-600 dark:text-dark-300">
              详细的使用统计，实时请求监控，成本分析报表，帮助您优化 API 使用
            </p>
          </div>

          <!-- Feature 6 -->
          <div class="rounded-xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-800 dark:bg-dark-900">
            <div class="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-lg bg-red-100 dark:bg-red-900/30">
              <svg class="h-6 w-6 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192l-3.536 3.536M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-5 0a4 4 0 11-8 0 4 4 0 018 0z" />
              </svg>
            </div>
            <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">多模型支持</h3>
            <p class="text-gray-600 dark:text-dark-300">
              支持 Claude 3.5、GPT-4、Gemini 等主流模型，一个 API 接入所有 AI 能力
            </p>
          </div>
        </div>
      </div>
    </section>

    <!-- Technical Features -->
    <section class="px-6 py-16">
      <div class="mx-auto max-w-6xl">
        <div class="text-center">
          <h2 class="text-3xl font-bold text-gray-900 dark:text-white">技术特性</h2>
          <p class="mt-4 text-lg text-gray-600 dark:text-dark-300">为开发者打造的专业 API 服务</p>
        </div>

        <div class="mt-12 grid gap-8 lg:grid-cols-2">
          <!-- Left Column -->
          <div class="space-y-6">
            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">RESTful API 设计</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">符合 OpenAI/Anthropic 标准接口，无缝迁移现有代码</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">流式响应支持</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">Server-Sent Events (SSE) 实时流式输出，提升用户体验</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">智能负载均衡</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">自动分配请求到最优节点，确保服务稳定高效</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">请求重试机制</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">自动重试失败请求，指数退避策略，提高成功率</p>
              </div>
            </div>
          </div>

          <!-- Right Column -->
          <div class="space-y-6">
            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">速率限制保护</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">灵活的速率限制配置，防止滥用，保护您的配额</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">详细错误信息</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">清晰的错误码和描述，快速定位和解决问题</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">Webhook 通知</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">余额预警、异常告警等事件实时推送到您的系统</p>
              </div>
            </div>

            <div class="flex gap-4">
              <div class="flex-shrink-0">
                <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary-100 dark:bg-primary-900/30">
                  <svg class="h-5 w-5 text-primary-600 dark:text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 class="font-semibold text-gray-900 dark:text-white">SDK 支持</h3>
                <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">兼容官方 SDK，Python、JavaScript、Go 等多语言支持</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- CTA Section -->
    <section class="px-6 py-16">
      <div class="mx-auto max-w-4xl">
        <div class="rounded-2xl bg-gradient-to-r from-primary-600 to-purple-600 px-8 py-12 text-center shadow-xl">
          <h2 class="text-3xl font-bold text-white">准备好开始了吗？</h2>
          <p class="mt-4 text-lg text-primary-100">
            立即注册，获取免费额度，体验强大的 AI API 服务
          </p>
          <div class="mt-8 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <router-link
              to="/register"
              class="rounded-lg bg-white px-8 py-3 font-semibold text-primary-600 shadow-lg hover:bg-gray-50"
            >
              免费注册
            </router-link>
            <router-link
              to="/docs"
              class="rounded-lg border-2 border-white px-8 py-3 font-semibold text-white hover:bg-white/10"
            >
              查看文档
            </router-link>
          </div>
        </div>
      </div>
    </section>

    <!-- Footer -->
    <footer class="border-t border-gray-200 px-6 py-8 dark:border-dark-800">
      <div class="mx-auto max-w-6xl text-center text-sm text-gray-500 dark:text-dark-400">
        <p>&copy; 2026 Sub2API. All rights reserved.</p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { useSEO, seoConfigs } from '@/composables/useSEO'

// SEO Configuration
useSEO(seoConfigs.features)
</script>
