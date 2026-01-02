<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Header (same as home) -->
    <header class="border-b border-gray-200 bg-white px-6 py-4 dark:border-dark-800 dark:bg-dark-900">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <router-link to="/home" class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img src="/logo.png" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-xl font-bold text-gray-900 dark:text-white">Sub2API</span>
        </router-link>

        <div class="flex items-center gap-4">
          <router-link to="/home" class="text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white">
            {{ t('common.home') || '首页' }}
          </router-link>
          <router-link to="/docs" class="text-gray-600 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white">
            {{ t('common.docs') || '文档' }}
          </router-link>
          <router-link
            to="/login"
            class="rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
          >
            {{ t('common.getStarted') || '开始使用' }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="px-6 py-16">
      <div class="mx-auto max-w-7xl">
        <!-- Page Header -->
        <div class="mb-16 text-center">
          <h1 class="mb-4 text-4xl font-bold text-gray-900 dark:text-white md:text-5xl">
            {{ t('pricing.title') || '灵活的定价方案' }}
          </h1>
          <p class="mx-auto max-w-2xl text-lg text-gray-600 dark:text-dark-400">
            {{ t('pricing.subtitle') || '选择最适合您的方案，随时可以升级或降级' }}
          </p>
        </div>

        <!-- Pricing Cards -->
        <div class="mb-16 grid gap-8 lg:grid-cols-3">
          <!-- Basic Plan -->
          <div class="rounded-2xl border-2 border-gray-200 bg-white p-8 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-8">
              <h3 class="mb-2 text-2xl font-bold text-gray-900 dark:text-white">
                {{ t('pricing.basic.name') || '按量付费' }}
              </h3>
              <p class="text-gray-600 dark:text-dark-400">
                {{ t('pricing.basic.subtitle') || '适合个人开发者和小型项目' }}
              </p>
            </div>

            <div class="mb-8">
              <div class="flex items-baseline">
                <span class="text-5xl font-bold text-gray-900 dark:text-white">$0</span>
                <span class="ml-2 text-gray-600 dark:text-dark-400">{{ t('pricing.basic.starting') }}</span>
              </div>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-500">
                {{ t('pricing.basic.billingNote') }}
              </p>
            </div>

            <ul class="mb-8 space-y-4">
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.basic.feature1') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.basic.feature2') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.basic.feature3') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.basic.feature4') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.basic.feature5') }}</span>
              </li>
            </ul>

            <router-link
              to="/register"
              class="block w-full rounded-lg border-2 border-primary-600 py-3 text-center font-semibold text-primary-600 transition-all hover:bg-primary-50 dark:border-primary-500 dark:text-primary-400 dark:hover:bg-primary-950"
            >
              {{ t('pricing.getStarted') || '免费开始' }}
            </router-link>
          </div>

          <!-- Pro Plan (Recommended) -->
          <div class="relative rounded-2xl border-2 border-primary-600 bg-white p-8 shadow-2xl dark:border-primary-500 dark:bg-dark-800">
            <div class="absolute -top-5 left-1/2 -translate-x-1/2 rounded-full bg-gradient-to-r from-primary-600 to-blue-600 px-6 py-2 text-sm font-bold text-white shadow-lg">
              {{ t('pricing.recommended') || '推荐' }}
            </div>

            <div class="mb-8">
              <h3 class="mb-2 text-2xl font-bold text-gray-900 dark:text-white">
                {{ t('pricing.pro.name') || '包月套餐' }}
              </h3>
              <p class="text-gray-600 dark:text-dark-400">
                {{ t('pricing.pro.subtitle') || '适合中小企业和成长型项目' }}
              </p>
            </div>

            <div class="mb-8">
              <div class="flex items-baseline">
                <span class="text-5xl font-bold text-gray-900 dark:text-white">$49</span>
                <span class="ml-2 text-gray-600 dark:text-dark-400">{{ t('pricing.pro.perMonth') }}</span>
              </div>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-500">
                {{ t('pricing.pro.billingNote') }}
              </p>
            </div>

            <ul class="mb-8 space-y-4">
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200" v-html="t('pricing.pro.feature1')"></span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.pro.feature2') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.pro.feature3') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.pro.feature4') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.pro.feature5') }}</span>
              </li>
            </ul>

            <router-link
              to="/register"
              class="block w-full rounded-lg bg-gradient-to-r from-primary-600 to-blue-600 py-3 text-center font-semibold text-white shadow-lg transition-all hover:shadow-xl"
            >
              {{ t('pricing.subscribe') || '立即订阅' }}
            </router-link>
          </div>

          <!-- Enterprise Plan -->
          <div class="rounded-2xl border-2 border-gray-200 bg-white p-8 dark:border-dark-700 dark:bg-dark-800">
            <div class="mb-8">
              <h3 class="mb-2 text-2xl font-bold text-gray-900 dark:text-white">
                {{ t('pricing.enterprise.name') || '企业定制' }}
              </h3>
              <p class="text-gray-600 dark:text-dark-400">
                {{ t('pricing.enterprise.subtitle') || '适合大型企业和高并发场景' }}
              </p>
            </div>

            <div class="mb-8">
              <div class="flex items-baseline">
                <span class="text-5xl font-bold text-gray-900 dark:text-white">{{ t('pricing.custom') || '定制' }}</span>
              </div>
              <p class="mt-2 text-sm text-gray-500 dark:text-dark-500">
                {{ t('pricing.enterprise.billingNote') }}
              </p>
            </div>

            <ul class="mb-8 space-y-4">
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200" v-html="t('pricing.enterprise.feature1')"></span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.enterprise.feature2') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.enterprise.feature3') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.enterprise.feature4') }}</span>
              </li>
              <li class="flex items-start gap-3">
                <svg class="mt-0.5 h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span class="text-gray-700 dark:text-dark-200">{{ t('pricing.enterprise.feature5') }}</span>
              </li>
            </ul>

            <a
              href="mailto:support@ai-in.one"
              class="block w-full rounded-lg border-2 border-primary-600 py-3 text-center font-semibold text-primary-600 transition-all hover:bg-primary-50 dark:border-primary-500 dark:text-primary-400 dark:hover:bg-primary-950"
            >
              {{ t('pricing.contactUs') || '联系我们' }}
            </a>
          </div>
        </div>

        <!-- FAQ Section -->
        <div class="mx-auto max-w-4xl">
          <h2 class="mb-8 text-center text-3xl font-bold text-gray-900 dark:text-white">
            {{ t('pricing.faq.title') || '定价常见问题' }}
          </h2>

          <div class="space-y-4">
            <details class="group rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between font-semibold text-gray-900 dark:text-white">
                <span>{{ t('pricing.faq.q1') || '如何计费？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="mt-4 text-gray-600 dark:text-dark-400">
                {{ t('pricing.faq.a1') || '按量付费根据实际使用的 tokens 数量计费；包月套餐包含固定额度，超出部分按优惠价计费。企业定制根据具体需求定价。' }}
              </div>
            </details>

            <details class="group rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between font-semibold text-gray-900 dark:text-white">
                <span>{{ t('pricing.faq.q2') || '可以随时升级或降级吗？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="mt-4 text-gray-600 dark:text-dark-400">
                {{ t('pricing.faq.a2') || '可以。您可以随时在控制台中升级或降级套餐。升级立即生效，降级在当前计费周期结束后生效。' }}
              </div>
            </details>

            <details class="group rounded-lg border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between font-semibold text-gray-900 dark:text-white">
                <span>{{ t('pricing.faq.q3') || '支持哪些支付方式？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="mt-4 text-gray-600 dark:text-dark-400">
                {{ t('pricing.faq.a3') || '目前支持兑换码充值。企业客户可以联系我们开通对公转账等支付方式。' }}
              </div>
            </details>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="border-t border-gray-200 px-6 py-8 dark:border-dark-800">
      <div class="mx-auto max-w-6xl text-center text-sm text-gray-500 dark:text-dark-400">
        <p>&copy; 2026 Sub2API. All rights reserved.</p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useSEO, seoConfigs } from '@/composables/useSEO'

const { t } = useI18n()

// SEO Configuration
useSEO(seoConfigs.pricing)
</script>
