<template>
  <div
    class="relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute -right-40 -top-40 h-96 w-96 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/3 top-1/4 h-72 w-72 rounded-full bg-primary-300/10 blur-3xl"
      ></div>
      <div
        class="absolute bottom-1/4 right-1/4 h-64 w-64 rounded-full bg-primary-400/10 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
        </div>

        <!-- Navigation Links -->
        <div class="hidden items-center gap-6 md:flex">
          <router-link
            to="/docs"
            class="text-sm font-medium text-gray-600 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('common.docs') }}
          </router-link>
          <router-link
            to="/pricing"
            class="text-sm font-medium text-gray-600 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('common.pricing') }}
          </router-link>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <svg
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 6.042A8.967 8.967 0 006 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 016 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 016-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0018 18a8.967 8.967 0 00-6 2.292m0-14.25v14.25"
              />
            </svg>
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <svg
              v-if="isDark"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z"
              />
            </svg>
            <svg
              v-else
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"
              />
            </svg>
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            to="/dashboard"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 px-6 py-16">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section - Enhanced Version -->
        <div class="mb-16 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <!-- Badge -->
            <div class="mb-6 inline-flex items-center gap-2 rounded-full bg-primary-50 px-4 py-2 dark:bg-primary-900/30">
              <span class="relative flex h-2 w-2">
                <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75"></span>
                <span class="relative inline-flex h-2 w-2 rounded-full bg-primary-500"></span>
              </span>
              <span class="text-sm font-medium text-primary-600 dark:text-primary-400">
                {{ t('home.hero.badge') || '正在为 1000+ 开发者提供服务' }}
              </span>
            </div>

            <!-- Main Heading -->
            <h1 class="mb-6 text-4xl font-bold text-gray-900 dark:text-white md:text-5xl lg:text-6xl">
              <span class="bg-gradient-to-r from-primary-600 to-blue-600 bg-clip-text text-transparent">
                Claude Code API
              </span>
              <br />
              {{ t('home.hero.title') || '中转服务' }}
            </h1>

            <!-- Subtitle with value propositions -->
            <p class="mb-4 text-lg text-gray-600 dark:text-dark-300 md:text-xl">
              {{ t('home.hero.subtitle') || '稳定、高效、按需付费的 AI API 接入方案' }}
            </p>

            <!-- Key Features List -->
            <ul class="mb-8 space-y-3 text-left">
              <li class="flex items-center gap-3 text-gray-700 dark:text-dark-200">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('home.hero.feature1') || '99.9% 可用性保证，7x24 稳定运行' }}</span>
              </li>
              <li class="flex items-center gap-3 text-gray-700 dark:text-dark-200">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('home.hero.feature2') || '无需繁琐注册，3 分钟即可开始使用' }}</span>
              </li>
              <li class="flex items-center gap-3 text-gray-700 dark:text-dark-200">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('home.hero.feature3') || '透明计费，按实际使用量付费' }}</span>
              </li>
            </ul>

            <!-- CTA Buttons -->
            <div class="flex flex-col gap-4 sm:flex-row sm:items-center">
              <router-link
                :to="isAuthenticated ? '/dashboard' : '/register'"
                class="inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-primary-600 to-blue-600 px-8 py-4 text-base font-semibold text-white shadow-lg shadow-primary-500/30 transition-all hover:shadow-xl hover:shadow-primary-500/40"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.hero.cta') || '免费开始使用' }}
                <svg
                  class="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M13.5 4.5L21 12m0 0l-7.5 7.5M21 12H3"
                  />
                </svg>
              </router-link>

              <router-link
                to="/pricing"
                class="inline-flex items-center justify-center gap-2 rounded-lg border-2 border-gray-300 bg-white px-8 py-4 text-base font-semibold text-gray-700 transition-all hover:border-primary-500 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:border-primary-500"
              >
                {{ t('home.hero.viewPricing') || '查看定价' }}
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
                </svg>
              </router-link>
            </div>

            <!-- Trust Indicators -->
            <div class="mt-8 flex flex-wrap items-center gap-6 text-sm text-gray-500 dark:text-dark-400">
              <div class="flex items-center gap-2">
                <svg class="h-5 w-5 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                </svg>
                <span>4.8/5.0 用户评分</span>
              </div>
              <div class="flex items-center gap-2">
                <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <span>99.9% 在线率</span>
              </div>
              <div class="flex items-center gap-2">
                <svg class="h-5 w-5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z" />
                </svg>
                <span>1000+ 开发者</span>
              </div>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">terminal</span>
                </div>
                <!-- Terminal content -->
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">export</span>
                    <span class="code-flag">ANTHROPIC_API_KEY</span>
                    <span class="code-operator">=</span>
                    <span class="code-string">sk-xxx</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">export</span>
                    <span class="code-flag">ANTHROPIC_BASE_URL</span>
                    <span class="code-operator">=</span>
                    <span class="code-url">https://api.ai-in.one</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">claude-code</span>
                    <span class="code-flag">chat</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-success">✓</span>
                    <span class="code-response">Connected to Claude API</span>
                  </div>
                  <div class="code-line line-5">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Feature Tags - Centered -->
        <div class="mb-12 flex flex-wrap items-center justify-center gap-4 md:gap-6">
          <div
            class="inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <svg
              class="h-4 w-4 text-primary-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5"
              />
            </svg>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.subscriptionToApi')
            }}</span>
          </div>
          <div
            class="inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <svg
              class="h-4 w-4 text-primary-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z"
              />
            </svg>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.stickySession')
            }}</span>
          </div>
          <div
            class="inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <svg
              class="h-4 w-4 text-primary-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"
              />
            </svg>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{
              t('home.tags.realtimeBilling')
            }}</span>
          </div>
        </div>

        <!-- Quick Start - 3 Steps -->
        <div class="mb-16">
          <div class="mb-8 text-center">
            <h2 class="mb-3 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">
              {{ t('home.quickStart.title') || '3 分钟快速开始' }}
            </h2>
            <p class="mx-auto max-w-2xl text-lg text-gray-600 dark:text-dark-400">
              {{ t('home.quickStart.subtitle') || '简单三步，立即接入 AI API 服务' }}
            </p>
          </div>

          <div class="grid gap-8 md:grid-cols-3">
            <!-- Step 1 -->
            <div class="relative">
              <div class="flex h-full flex-col rounded-2xl border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-blue-500 text-xl font-bold text-white">
                  1
                </div>
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickStart.step1.title') || '注册账号' }}
                </h3>
                <p class="mb-4 flex-1 text-gray-600 dark:text-dark-400">
                  {{ t('home.quickStart.step1.desc') || '使用邮箱快速注册，免费获得试用额度' }}
                </p>
                <div class="flex items-center gap-2 text-sm text-primary-600 dark:text-primary-400">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>30 秒完成注册</span>
                </div>
              </div>
            </div>

            <!-- Step 2 -->
            <div class="relative">
              <div class="flex h-full flex-col rounded-2xl border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-purple-500 to-pink-500 text-xl font-bold text-white">
                  2
                </div>
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickStart.step2.title') || '生成 API Key' }}
                </h3>
                <p class="mb-4 flex-1 text-gray-600 dark:text-dark-400">
                  {{ t('home.quickStart.step2.desc') || '在控制台一键生成专属 API 密钥' }}
                </p>
                <div class="flex items-center gap-2 text-sm text-primary-600 dark:text-primary-400">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>支持多个 Key 管理</span>
                </div>
              </div>
            </div>

            <!-- Step 3 -->
            <div class="relative">
              <div class="flex h-full flex-col rounded-2xl border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
                <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-green-500 to-emerald-500 text-xl font-bold text-white">
                  3
                </div>
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.quickStart.step3.title') || '开始调用' }}
                </h3>
                <p class="mb-4 flex-1 text-gray-600 dark:text-dark-400">
                  {{ t('home.quickStart.step3.desc') || '复制示例代码，替换 API Key 即可使用' }}
                </p>
                <div class="flex items-center gap-2 text-sm text-primary-600 dark:text-primary-400">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>完整的 API 文档</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Pricing Preview -->
        <div class="mb-16">
          <div class="mb-8 text-center">
            <h2 class="mb-3 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">
              {{ t('home.pricing.title') || '灵活的定价方案' }}
            </h2>
            <p class="mx-auto max-w-2xl text-lg text-gray-600 dark:text-dark-400">
              {{ t('home.pricing.subtitle') || '按需付费，透明计费，无隐藏费用' }}
            </p>
          </div>

          <div class="grid gap-6 md:grid-cols-3">
            <!-- Basic Plan -->
            <div class="rounded-2xl border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4">
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.pricing.basic.name') || '按量付费' }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-dark-400">
                  {{ t('home.pricing.basic.desc') || '适合个人开发者和小型项目' }}
                </p>
              </div>
              <div class="mb-6">
                <div class="flex items-baseline">
                  <span class="text-4xl font-bold text-gray-900 dark:text-white">$0</span>
                  <span class="ml-2 text-gray-600 dark:text-dark-400">起步</span>
                </div>
              </div>
              <ul class="mb-6 space-y-3">
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>按实际使用量计费</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>无最低消费</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>基础技术支持</span>
                </li>
              </ul>
              <router-link
                to="/pricing"
                class="block w-full rounded-lg border-2 border-gray-300 py-3 text-center font-semibold text-gray-700 transition-all hover:border-primary-500 hover:text-primary-600 dark:border-dark-600 dark:text-dark-200 dark:hover:border-primary-500"
              >
                {{ t('home.pricing.viewDetails') || '查看详情' }}
              </router-link>
            </div>

            <!-- Pro Plan -->
            <div class="relative rounded-2xl border-2 border-primary-500 bg-white p-6 shadow-xl dark:bg-dark-800">
              <div class="absolute -top-4 left-1/2 -translate-x-1/2 rounded-full bg-gradient-to-r from-primary-600 to-blue-600 px-4 py-1 text-sm font-semibold text-white">
                {{ t('home.pricing.popular') || '推荐' }}
              </div>
              <div class="mb-4">
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.pricing.pro.name') || '包月套餐' }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-dark-400">
                  {{ t('home.pricing.pro.desc') || '适合中小企业和成长型项目' }}
                </p>
              </div>
              <div class="mb-6">
                <div class="flex items-baseline">
                  <span class="text-4xl font-bold text-gray-900 dark:text-white">$20</span>
                  <span class="ml-2 text-gray-600 dark:text-dark-400">/月</span>
                </div>
              </div>
              <ul class="mb-6 space-y-3">
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>每月 2000 万 tokens</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>更高并发限制</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>优先技术支持</span>
                </li>
              </ul>
              <router-link
                to="/pricing"
                class="block w-full rounded-lg bg-gradient-to-r from-primary-600 to-blue-600 py-3 text-center font-semibold text-white shadow-lg transition-all hover:shadow-xl"
              >
                {{ t('home.pricing.getStarted') || '立即开始' }}
              </router-link>
            </div>

            <!-- Enterprise Plan -->
            <div class="rounded-2xl border border-gray-200 bg-white p-6 dark:border-dark-700 dark:bg-dark-800">
              <div class="mb-4">
                <h3 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
                  {{ t('home.pricing.enterprise.name') || '企业定制' }}
                </h3>
                <p class="text-sm text-gray-600 dark:text-dark-400">
                  {{ t('home.pricing.enterprise.desc') || '适合大型企业和高并发场景' }}
                </p>
              </div>
              <div class="mb-6">
                <div class="flex items-baseline">
                  <span class="text-4xl font-bold text-gray-900 dark:text-white">{{ t('home.pricing.custom') || '定制' }}</span>
                </div>
              </div>
              <ul class="mb-6 space-y-3">
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>无限额度</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>专属客户经理</span>
                </li>
                <li class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
                  <svg class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                  </svg>
                  <span>SLA 保障</span>
                </li>
              </ul>
              <router-link
                to="/pricing"
                class="block w-full rounded-lg border-2 border-gray-300 py-3 text-center font-semibold text-gray-700 transition-all hover:border-primary-500 hover:text-primary-600 dark:border-dark-600 dark:text-dark-200 dark:hover:border-primary-500"
              >
                {{ t('home.pricing.contactUs') || '联系我们' }}
              </router-link>
            </div>
          </div>
        </div>

        <!-- FAQ Section -->
        <div class="mb-16">
          <div class="mb-8 text-center">
            <h2 class="mb-3 text-3xl font-bold text-gray-900 dark:text-white md:text-4xl">
              {{ t('home.faq.title') || '常见问题' }}
            </h2>
            <p class="mx-auto max-w-2xl text-lg text-gray-600 dark:text-dark-400">
              {{ t('home.faq.subtitle') || '快速找到您关心的问题' }}
            </p>
          </div>

          <div class="mx-auto max-w-3xl space-y-4">
            <!-- FAQ Item 1 -->
            <details class="group rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between p-6 font-semibold text-gray-900 dark:text-white">
                <span>{{ t('home.faq.q1') || '如何计费？按什么标准收费？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="border-t border-gray-200 p-6 pt-4 text-gray-600 dark:border-dark-700 dark:text-dark-400">
                {{ t('home.faq.a1') || '我们按照实际使用的 tokens 数量计费，价格透明，无隐藏费用。支持按量付费和包月套餐两种方式，您可以根据实际需求选择。' }}
              </div>
            </details>

            <!-- FAQ Item 2 -->
            <details class="group rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between p-6 font-semibold text-gray-900 dark:text-white">
                <span>{{ t('home.faq.q2') || '支持哪些 AI 模型？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="border-t border-gray-200 p-6 pt-4 text-gray-600 dark:border-dark-700 dark:text-dark-400">
                {{ t('home.faq.a2') || '目前支持 Claude、GPT、Gemini 等主流 AI 模型。我们会持续扩展支持的模型，敬请期待。' }}
              </div>
            </details>

            <!-- FAQ Item 3 -->
            <details class="group rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between p-6 font-semibold text-gray-900 dark:text-white">
                <span>{{ t('home.faq.q3') || '有使用限制吗？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="border-t border-gray-200 p-6 pt-4 text-gray-600 dark:border-dark-700 dark:text-dark-400">
                {{ t('home.faq.a3') || '根据不同套餐有不同的并发限制和额度限制。按量付费套餐有基础并发限制，包月和企业套餐享有更高的并发和额度。' }}
              </div>
            </details>

            <!-- FAQ Item 4 -->
            <details class="group rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between p-6 font-semibold text-gray-900 dark:text-white">
                <span>{{ t('home.faq.q4') || '如何获取 API Key？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="border-t border-gray-200 p-6 pt-4 text-gray-600 dark:border-dark-700 dark:text-dark-400">
                {{ t('home.faq.a4') || '注册并登录后，在控制台的 API Keys 页面可以一键生成。支持创建多个 Key，方便管理不同项目。' }}
              </div>
            </details>

            <!-- FAQ Item 5 -->
            <details class="group rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
              <summary class="flex cursor-pointer items-center justify-between p-6 font-semibold text-gray-900 dark:text-white">
                <span>{{ t('home.faq.q5') || '服务稳定性如何保障？' }}</span>
                <svg class="h-5 w-5 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </summary>
              <div class="border-t border-gray-200 p-6 pt-4 text-gray-600 dark:border-dark-700 dark:text-dark-400">
                {{ t('home.faq.a5') || '我们承诺 99.9% 的服务可用性，采用多账号池技术，即使单个账号出现问题也能自动切换，确保服务不间断。' }}
              </div>
            </details>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPublicSettings } from '@/api/auth'
import { useAuthStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { useSEO, seoConfigs } from '@/composables/useSEO'
import { useOrganizationSchema, useWebSiteSchema } from '@/composables/useStructuredData'

const { t } = useI18n()

// SEO Configuration
useSEO(seoConfigs.home)

// Structured Data
useOrganizationSchema({
  name: 'Sub2API',
  url: 'https://ai-in.one',
  logo: 'https://ai-in.one/logo.png',
  description: 'Claude Code API 中转服务 - 稳定、高效、按需付费的 AI API 接入方案',
  sameAs: [
    'https://github.com/Wei-Shaw/sub2api'
  ]
})

useWebSiteSchema('https://ai-in.one', 'Sub2API - AI API Gateway')

const authStore = useAuthStore()

// Site settings
const siteName = ref('Sub2API')
const siteLogo = ref('')
const siteSubtitle = ref('AI API Gateway Platform')
const docUrl = ref('')

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(async () => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  try {
    const settings = await getPublicSettings()
    siteName.value = settings.site_name || 'Sub2API'
    siteLogo.value = settings.site_logo || ''
    siteSubtitle.value = settings.site_subtitle || 'AI API Gateway Platform'
    docUrl.value = settings.doc_url || ''
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
})
</script>

<style scoped>
/* Terminal Container */
.terminal-container {
  position: relative;
  display: inline-block;
}

/* Terminal Window */
.terminal-window {
  width: 560px;
  background: linear-gradient(145deg, #1e293b 0%, #0f172a 100%);
  border-radius: 14px;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  overflow: hidden;
  transform: perspective(1000px) rotateX(2deg) rotateY(-2deg);
  transition: transform 0.3s ease;
}

.terminal-window:hover {
  transform: perspective(1000px) rotateX(0deg) rotateY(0deg) translateY(-4px);
}

/* Terminal Header */
.terminal-header {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  background: rgba(30, 41, 59, 0.8);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #ef4444;
}
.btn-minimize {
  background: #eab308;
}
.btn-maximize {
  background: #22c55e;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  color: #64748b;
  margin-right: 52px;
}

/* Terminal Body */
.terminal-body {
  padding: 20px 24px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 14px;
  line-height: 2;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 {
  animation-delay: 0.3s;
}
.line-2 {
  animation-delay: 1s;
}
.line-3 {
  animation-delay: 1.8s;
}
.line-4 {
  animation-delay: 2.5s;
}
.line-5 {
  animation-delay: 3.2s;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #22c55e;
  font-weight: bold;
}
.code-cmd {
  color: #38bdf8;
}
.code-flag {
  color: #a78bfa;
}
.code-url {
  color: #14b8a6;
}
.code-operator {
  color: #f59e0b;
}
.code-string {
  color: #fbbf24;
}
.code-comment {
  color: #64748b;
  font-style: italic;
}
.code-success {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response {
  color: #fbbf24;
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(20, 184, 166, 0.2),
    0 0 40px rgba(20, 184, 166, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
</style>
