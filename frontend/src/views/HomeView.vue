<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page - AI Tech Theme -->
  <div
    v-else
    class="relative flex min-h-screen flex-col overflow-hidden bg-[#0a0e1a]"
  >
    <!-- Animated Background -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <!-- Grid pattern -->
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.04)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.04)_1px,transparent_1px)] bg-[size:48px_48px]"
      ></div>
      <!-- Glow orbs -->
      <div class="absolute -right-32 -top-32 h-[500px] w-[500px] rounded-full bg-primary-500/10 blur-[120px] animate-pulse-slow"></div>
      <div class="absolute -bottom-32 -left-32 h-[400px] w-[400px] rounded-full bg-blue-500/8 blur-[100px] animate-pulse-slow" style="animation-delay: 1s;"></div>
      <div class="absolute left-1/2 top-1/3 h-[300px] w-[300px] rounded-full bg-primary-400/5 blur-[80px] animate-pulse-slow" style="animation-delay: 2s;"></div>
      <!-- Floating particles -->
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
      <div class="particle particle-4"></div>
      <div class="particle particle-5"></div>
      <div class="particle particle-6"></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md ring-1 ring-white/10">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <span class="text-lg font-bold text-white">{{ siteName }}</span>
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
            class="rounded-lg p-2 text-gray-400 transition-colors hover:bg-white/5 hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-400 transition-colors hover:bg-white/5 hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Login / Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-4 py-2 text-sm font-medium text-white shadow-lg shadow-primary-500/25 transition-all hover:shadow-primary-500/40 hover:scale-105"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-white/20 text-[10px] font-semibold"
            >
              {{ userInitial }}
            </span>
            <span>{{ t('home.dashboard') }}</span>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-5 py-2 text-sm font-medium text-white shadow-lg shadow-primary-500/25 transition-all hover:shadow-primary-500/40 hover:scale-105"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6">
      <!-- Hero Section -->
      <section class="mx-auto max-w-6xl pt-16 pb-20">
        <div class="flex flex-col items-center text-center">
          <!-- Badge -->
          <div class="mb-6 inline-flex items-center gap-2 rounded-full border border-primary-500/30 bg-primary-500/10 px-4 py-1.5 backdrop-blur-sm">
            <span class="relative flex h-2 w-2">
              <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75"></span>
              <span class="relative inline-flex h-2 w-2 rounded-full bg-primary-500"></span>
            </span>
            <span class="text-xs font-medium text-primary-300">GPT API 稳定中转服务</span>
          </div>

          <!-- Title -->
          <h1 class="mb-6 text-4xl font-bold leading-tight text-white md:text-5xl lg:text-6xl">
            <span class="bg-gradient-to-r from-white via-white to-gray-300 bg-clip-text text-transparent">{{ siteName }}</span>
            <br />
            <span class="bg-gradient-to-r from-primary-400 to-cyan-400 bg-clip-text text-transparent">智能 AI 中转站</span>
          </h1>

          <!-- Subtitle -->
          <p class="mb-8 max-w-2xl text-lg text-gray-400 md:text-xl">
            专注 GPT 系列模型的稳定中转服务，多节点负载均衡，99.9% 可用性保障，让你的 AI 应用永不掉线
          </p>

          <!-- CTA Buttons -->
          <div class="flex flex-wrap items-center justify-center gap-4">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="group inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-8 py-3.5 text-base font-semibold text-white shadow-xl shadow-primary-500/30 transition-all hover:shadow-primary-500/50 hover:scale-105"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="md" class="transition-transform group-hover:translate-x-1" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-8 py-3.5 text-base font-medium text-gray-300 backdrop-blur-sm transition-all hover:bg-white/10 hover:text-white"
            >
              <Icon name="book" size="md" />
              查看文档
            </a>
          </div>

          <!-- Stats -->
          <div class="mt-12 grid grid-cols-3 gap-8 md:gap-16">
            <div class="text-center">
              <div class="text-2xl font-bold text-white md:text-3xl">99.9%</div>
              <div class="mt-1 text-sm text-gray-500">可用性</div>
            </div>
            <div class="text-center">
              <div class="text-2xl font-bold text-white md:text-3xl">&lt;100ms</div>
              <div class="mt-1 text-sm text-gray-500">平均延迟</div>
            </div>
            <div class="text-center">
              <div class="text-2xl font-bold text-white md:text-3xl">24/7</div>
              <div class="mt-1 text-sm text-gray-500">全天候服务</div>
            </div>
          </div>
        </div>
      </section>

      <!-- Features Section -->
      <section class="mx-auto max-w-6xl py-16">
        <div class="mb-12 text-center">
          <h2 class="mb-3 text-3xl font-bold text-white">为什么选择我们</h2>
          <p class="text-gray-400">专业的 GPT API 中转解决方案</p>
        </div>

        <div class="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
          <!-- Feature 1 -->
          <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-6 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:bg-white/[0.05]">
            <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500/20 to-primary-600/20 ring-1 ring-primary-500/30">
              <Icon name="shield" size="lg" class="text-primary-400" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">稳定可靠</h3>
            <p class="text-sm leading-relaxed text-gray-400">
              多节点智能负载均衡，自动故障转移，确保服务 99.9% 可用性
            </p>
          </div>

          <!-- Feature 2 -->
          <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-6 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:bg-white/[0.05]">
            <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500/20 to-blue-600/20 ring-1 ring-blue-500/30">
              <Icon name="bolt" size="lg" class="text-blue-400" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">极速响应</h3>
            <p class="text-sm leading-relaxed text-gray-400">
              优化网络链路，全球加速节点，API 请求延迟低至毫秒级
            </p>
          </div>

          <!-- Feature 3 -->
          <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-6 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:bg-white/[0.05]">
            <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-purple-500/20 to-purple-600/20 ring-1 ring-purple-500/30">
              <Icon name="key" size="lg" class="text-purple-400" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">简单接入</h3>
            <p class="text-sm leading-relaxed text-gray-400">
              兼容 OpenAI API 格式，只需替换 Base URL 即可无缝切换
            </p>
          </div>

          <!-- Feature 4 -->
          <div class="group rounded-2xl border border-white/5 bg-white/[0.02] p-6 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:bg-white/[0.05]">
            <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500/20 to-amber-600/20 ring-1 ring-amber-500/30">
              <Icon name="chart" size="lg" class="text-amber-400" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">用量透明</h3>
            <p class="text-sm leading-relaxed text-gray-400">
              实时用量统计，详细账单明细，每一笔消费清晰可见
            </p>
          </div>
        </div>
      </section>

      <!-- Supported Models Section -->
      <section class="mx-auto max-w-6xl py-16">
        <div class="mb-12 text-center">
          <h2 class="mb-3 text-3xl font-bold text-white">支持的模型</h2>
          <p class="text-gray-400">当前专注于 GPT 系列，提供最佳中转体验</p>
        </div>

        <div class="flex flex-wrap items-center justify-center gap-4">
          <!-- GPT-4o -->
          <div class="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-5 py-3 ring-1 ring-primary-500/10">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-emerald-600 shadow-lg shadow-green-500/20">
              <span class="text-sm font-bold text-white">G</span>
            </div>
            <div>
              <span class="text-sm font-semibold text-white">GPT-4o</span>
              <span class="ml-2 rounded bg-primary-500/20 px-1.5 py-0.5 text-[10px] font-medium text-primary-400">可用</span>
            </div>
          </div>

          <!-- GPT-4o-mini -->
          <div class="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-5 py-3 ring-1 ring-primary-500/10">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-emerald-600 shadow-lg shadow-green-500/20">
              <span class="text-sm font-bold text-white">G</span>
            </div>
            <div>
              <span class="text-sm font-semibold text-white">GPT-4o-mini</span>
              <span class="ml-2 rounded bg-primary-500/20 px-1.5 py-0.5 text-[10px] font-medium text-primary-400">可用</span>
            </div>
          </div>

          <!-- GPT-4.1 -->
          <div class="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-5 py-3 ring-1 ring-primary-500/10">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-emerald-600 shadow-lg shadow-green-500/20">
              <span class="text-sm font-bold text-white">G</span>
            </div>
            <div>
              <span class="text-sm font-semibold text-white">GPT-4.1</span>
              <span class="ml-2 rounded bg-primary-500/20 px-1.5 py-0.5 text-[10px] font-medium text-primary-400">可用</span>
            </div>
          </div>

          <!-- o1 -->
          <div class="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-5 py-3 ring-1 ring-primary-500/10">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-emerald-600 shadow-lg shadow-green-500/20">
              <span class="text-sm font-bold text-white">G</span>
            </div>
            <div>
              <span class="text-sm font-semibold text-white">o1 / o3</span>
              <span class="ml-2 rounded bg-primary-500/20 px-1.5 py-0.5 text-[10px] font-medium text-primary-400">可用</span>
            </div>
          </div>

          <!-- Codex -->
          <div class="flex items-center gap-3 rounded-xl border border-primary-500/30 bg-primary-500/5 px-5 py-3 ring-1 ring-primary-500/10">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-green-500 to-emerald-600 shadow-lg shadow-green-500/20">
              <span class="text-sm font-bold text-white">C</span>
            </div>
            <div>
              <span class="text-sm font-semibold text-white">Codex</span>
              <span class="ml-2 rounded bg-primary-500/20 px-1.5 py-0.5 text-[10px] font-medium text-primary-400">可用</span>
            </div>
          </div>

          <!-- More -->
          <div class="flex items-center gap-3 rounded-xl border border-white/10 bg-white/[0.02] px-5 py-3">
            <div class="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-gray-600 to-gray-700">
              <span class="text-sm font-bold text-white">+</span>
            </div>
            <div>
              <span class="text-sm font-medium text-gray-400">更多模型</span>
              <span class="ml-2 rounded bg-white/10 px-1.5 py-0.5 text-[10px] font-medium text-gray-500">持续更新</span>
            </div>
          </div>
        </div>
      </section>

      <!-- Pricing Section - Monthly Card -->
      <section class="mx-auto max-w-6xl py-16" id="pricing">
        <div class="mb-12 text-center">
          <h2 class="mb-3 text-3xl font-bold text-white">选择套餐</h2>
          <p class="text-gray-400">灵活的计费方式，满足不同使用需求</p>
        </div>

        <div class="grid gap-6 md:grid-cols-3">
          <!-- Plan 1: 基础月卡 -->
          <div class="relative rounded-2xl border border-white/10 bg-gradient-to-b from-white/[0.05] to-transparent p-8 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:shadow-lg hover:shadow-primary-500/5">
            <div class="mb-6">
              <h3 class="text-xl font-bold text-white">基础月卡</h3>
              <p class="mt-1 text-sm text-gray-400">适合个人轻度使用</p>
            </div>
            <div class="mb-6">
              <span class="text-4xl font-bold text-white">¥50</span>
              <span class="text-gray-400">/月</span>
            </div>
            <ul class="mb-8 space-y-3">
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                200 美金额度/月
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                支持 GPT-4o-mini
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                基础速率限制
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                用量统计面板
              </li>
            </ul>
            <router-link
              :to="isAuthenticated ? '/payment' : '/login'"
              class="block w-full rounded-xl border border-primary-500/30 bg-primary-500/10 py-3 text-center text-sm font-semibold text-primary-400 transition-all hover:bg-primary-500/20"
            >
              立即订阅
            </router-link>
          </div>

          <!-- Plan 2: 专业月卡 (Popular) -->
          <div class="relative rounded-2xl border border-primary-500/50 bg-gradient-to-b from-primary-500/10 to-transparent p-8 backdrop-blur-sm shadow-xl shadow-primary-500/10 transition-all duration-300 hover:shadow-primary-500/20">
            <!-- Popular badge -->
            <div class="absolute -top-3 left-1/2 -translate-x-1/2">
              <span class="rounded-full bg-gradient-to-r from-primary-500 to-cyan-500 px-4 py-1 text-xs font-bold text-white shadow-lg shadow-primary-500/30">
                最受欢迎
              </span>
            </div>
            <div class="mb-6">
              <h3 class="text-xl font-bold text-white">专业月卡</h3>
              <p class="mt-1 text-sm text-gray-400">适合开发者日常使用</p>
            </div>
            <div class="mb-6">
              <span class="text-4xl font-bold text-white">¥200</span>
              <span class="text-gray-400">/月</span>
            </div>
            <ul class="mb-8 space-y-3">
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                1000 美金额度/月
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                支持全部 GPT 模型
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                更高速率限制
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                优先队列调度
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                详细用量分析
              </li>
            </ul>
            <router-link
              :to="isAuthenticated ? '/payment' : '/login'"
              class="block w-full rounded-xl bg-gradient-to-r from-primary-500 to-primary-600 py-3 text-center text-sm font-semibold text-white shadow-lg shadow-primary-500/30 transition-all hover:shadow-primary-500/50 hover:scale-[1.02]"
            >
              立即订阅
            </router-link>
          </div>

          <!-- Plan 3: 尊享月卡 -->
          <div class="relative rounded-2xl border border-white/10 bg-gradient-to-b from-white/[0.05] to-transparent p-8 backdrop-blur-sm transition-all duration-300 hover:border-primary-500/30 hover:shadow-lg hover:shadow-primary-500/5">
            <div class="mb-6">
              <h3 class="text-xl font-bold text-white">尊享月卡</h3>
              <p class="mt-1 text-sm text-gray-400">适合团队与高频使用</p>
            </div>
            <div class="mb-6">
              <span class="text-4xl font-bold text-white">¥500</span>
              <span class="text-gray-400">/月</span>
            </div>
            <ul class="mb-8 space-y-3">
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                3000 美金额度/月
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                支持全部 GPT 模型
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                无速率限制
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                专属高速通道
              </li>
              <li class="flex items-center gap-2 text-sm text-gray-300">
                <Icon name="check" size="sm" class="text-primary-400" :stroke-width="3" />
                优先技术支持
              </li>
            </ul>
            <router-link
              :to="isAuthenticated ? '/payment' : '/login'"
              class="block w-full rounded-xl border border-primary-500/30 bg-primary-500/10 py-3 text-center text-sm font-semibold text-primary-400 transition-all hover:bg-primary-500/20"
            >
              立即订阅
            </router-link>
          </div>
        </div>

        <!-- Additional note -->
        <div class="mt-8 text-center">
          <p class="text-sm text-gray-500">
            所有套餐均支持按量计费模式 · 额度用完可随时加购 · 未使用额度不过期
          </p>
        </div>
      </section>

      <!-- How it works -->
      <section class="mx-auto max-w-6xl py-16">
        <div class="mb-12 text-center">
          <h2 class="mb-3 text-3xl font-bold text-white">三步开始使用</h2>
          <p class="text-gray-400">兼容 OpenAI 官方 API 格式，无缝切换</p>
        </div>

        <div class="grid gap-8 md:grid-cols-3">
          <div class="text-center">
            <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-primary-500/20 to-primary-600/20 ring-1 ring-primary-500/30">
              <span class="text-xl font-bold text-primary-400">1</span>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">注册账号</h3>
            <p class="text-sm text-gray-400">创建账号并选择适合的套餐</p>
          </div>
          <div class="text-center">
            <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-primary-500/20 to-primary-600/20 ring-1 ring-primary-500/30">
              <span class="text-xl font-bold text-primary-400">2</span>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">获取密钥</h3>
            <p class="text-sm text-gray-400">在控制台生成你的 API Key</p>
          </div>
          <div class="text-center">
            <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-primary-500/20 to-primary-600/20 ring-1 ring-primary-500/30">
              <span class="text-xl font-bold text-primary-400">3</span>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-white">开始调用</h3>
            <p class="text-sm text-gray-400">替换 Base URL 即可使用</p>
          </div>
        </div>

        <!-- Code Example -->
        <div class="mt-12 mx-auto max-w-2xl">
          <div class="terminal-window">
            <!-- Window header -->
            <div class="terminal-header">
              <div class="terminal-buttons">
                <span class="btn-close"></span>
                <span class="btn-minimize"></span>
                <span class="btn-maximize"></span>
              </div>
              <span class="terminal-title">API 调用示例</span>
            </div>
            <!-- Terminal content -->
            <div class="terminal-body">
              <div class="code-line line-1">
                <span class="code-prompt">$</span>
                <span class="code-cmd">curl</span>
                <span class="code-flag">-X POST</span>
                <span class="code-url">{{ siteName }}/v1/chat/completions</span>
              </div>
              <div class="code-line line-2">
                <span class="code-flag">-H</span>
                <span class="code-response">"Authorization: Bearer sk-xxx"</span>
              </div>
              <div class="code-line line-3">
                <span class="code-flag">-d</span>
                <span class="code-response">'{"model": "gpt-4o", "messages": [...]}'</span>
              </div>
              <div class="code-line line-4">
                <span class="code-comment"># Routing to upstream...</span>
              </div>
              <div class="code-line line-5">
                <span class="code-success">200 OK</span>
                <span class="code-response">{ "choices": [...] }</span>
              </div>
              <div class="code-line line-6">
                <span class="code-prompt">$</span>
                <span class="cursor"></span>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- CTA Section -->
      <section class="mx-auto max-w-6xl py-16">
        <div class="relative overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-r from-primary-500/10 via-transparent to-cyan-500/10 p-12 text-center backdrop-blur-sm">
          <div class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:32px_32px]"></div>
          <div class="relative z-10">
            <h2 class="mb-4 text-3xl font-bold text-white">准备好开始了吗？</h2>
            <p class="mb-8 text-gray-400">注册即可体验稳定高速的 GPT API 中转服务</p>
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex items-center gap-2 rounded-full bg-gradient-to-r from-primary-500 to-primary-600 px-10 py-4 text-base font-semibold text-white shadow-xl shadow-primary-500/30 transition-all hover:shadow-primary-500/50 hover:scale-105"
            >
              {{ isAuthenticated ? '进入控制台' : '免费注册' }}
              <Icon name="arrowRight" size="md" :stroke-width="2" />
            </router-link>
          </div>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-white/5 px-6 py-8">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-gray-500">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-primary-400"
          >
            {{ t('home.docs') }}
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme - always dark for this page
const isDark = ref(true)

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
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
  // Default to dark theme for this page
  isDark.value = true
  document.documentElement.classList.add('dark')
  localStorage.setItem('theme', 'dark')
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* Floating Particles */
.particle {
  position: absolute;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: rgba(20, 184, 166, 0.4);
  animation: float-particle 15s infinite linear;
}

.particle-1 {
  top: 20%;
  left: 10%;
  animation-delay: 0s;
  animation-duration: 20s;
}

.particle-2 {
  top: 60%;
  left: 80%;
  animation-delay: 3s;
  animation-duration: 18s;
  width: 3px;
  height: 3px;
}

.particle-3 {
  top: 40%;
  left: 50%;
  animation-delay: 6s;
  animation-duration: 22s;
  width: 2px;
  height: 2px;
  background: rgba(56, 189, 248, 0.3);
}

.particle-4 {
  top: 80%;
  left: 30%;
  animation-delay: 9s;
  animation-duration: 16s;
  width: 3px;
  height: 3px;
  background: rgba(20, 184, 166, 0.3);
}

.particle-5 {
  top: 10%;
  left: 70%;
  animation-delay: 12s;
  animation-duration: 25s;
  width: 2px;
  height: 2px;
  background: rgba(139, 92, 246, 0.3);
}

.particle-6 {
  top: 50%;
  left: 20%;
  animation-delay: 4s;
  animation-duration: 19s;
  width: 3px;
  height: 3px;
  background: rgba(56, 189, 248, 0.25);
}

@keyframes float-particle {
  0% {
    transform: translate(0, 0) scale(1);
    opacity: 0;
  }
  10% {
    opacity: 1;
  }
  90% {
    opacity: 1;
  }
  100% {
    transform: translate(100px, -200px) scale(0.5);
    opacity: 0;
  }
}

/* Terminal Window */
.terminal-window {
  background: linear-gradient(145deg, #1e293b 0%, #0f172a 100%);
  border-radius: 14px;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.08),
    0 0 40px rgba(20, 184, 166, 0.05),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  overflow: hidden;
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
  font-size: 13px;
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

.line-1 { animation-delay: 0.3s; }
.line-2 { animation-delay: 0.8s; }
.line-3 { animation-delay: 1.3s; }
.line-4 { animation-delay: 1.8s; }
.line-5 { animation-delay: 2.5s; }
.line-6 { animation-delay: 3.2s; }

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
  0%, 50% {
    opacity: 1;
  }
  51%, 100% {
    opacity: 0;
  }
}
</style>
