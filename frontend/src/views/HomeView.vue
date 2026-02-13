<template>
  <!-- Opening Animation Overlay -->
  <div
    v-if="showAnimation"
    class="fixed inset-0 z-50 overflow-hidden pointer-events-none"
    :class="isDark ? 'bg-dark-950/50' : 'bg-gray-50/50'"
  >
    <!-- Car (clickable for horn sound) - Light theme -->
    <picture v-show="!isDark" class="car-animation absolute h-20 w-auto cursor-pointer pointer-events-auto" :style="carStyle">
      <source srcset="/car.webp" type="image/webp" />
      <img
        ref="carRef"
        src="/car.png"
        alt="car"
        class="h-full w-auto"
        @click="playHorn"
      />
    </picture>
    <!-- Car (clickable for horn sound) - Dark theme -->
    <picture v-show="isDark" class="car-animation absolute h-20 w-auto cursor-pointer pointer-events-auto" :style="carStyle">
      <source srcset="/car_night.webp" type="image/webp" />
      <img
        ref="carRef"
        src="/car_night.png"
        alt="car"
        class="h-full w-auto"
        @click="playHorn"
      />
    </picture>
    <!-- Falling Logos -->
    <img
      v-for="(logo, index) in fallingLogos"
      :key="index"
      :src="logo.src"
      :alt="logo.name"
      class="absolute h-10 w-10 object-contain rounded-lg shadow-lg"
      :style="logo.style"
    />
  </div>

  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
      sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
    ></iframe>
    <!-- HTML mode - Sanitized to prevent XSS attacks -->
    <div v-else v-html="sanitizedHomeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
  >
    <!-- Background Decorations -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="absolute -right-40 -top-40 h-96 w-96 rounded-full bg-primary-400/20 blur-3xl"></div>
      <div class="absolute -bottom-40 -left-40 h-96 w-96 rounded-full bg-primary-500/15 blur-3xl"></div>
      <div class="absolute left-1/3 top-1/4 h-72 w-72 rounded-full bg-primary-300/10 blur-3xl"></div>
      <div class="absolute bottom-1/4 right-1/4 h-64 w-64 rounded-full bg-primary-400/10 blur-3xl"></div>
      <div class="absolute inset-0 bg-[linear-gradient(rgba(217,119,87,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(217,119,87,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"></div>
    </div>

    <!-- Sticky Nav (hidden during opening animation, fade in after) -->
    <StickyNav
      ref="stickyNavRef"
      class="transition-opacity duration-500"
      :class="showAnimation ? 'opacity-0' : 'opacity-100'"
    />

    <!-- Hero Section -->
    <section class="relative z-10 px-6 py-16 sm:py-20">
      <div class="mx-auto max-w-6xl">
        <div class="flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div
            class="flex-1 text-center lg:text-left transition-all duration-700"
            :class="showHeroContent ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-8'"
          >
            <!-- Hidden animation target (invisible, for car to fly toward) -->
            <div
              ref="siteLogoRef"
              class="h-0 w-0 overflow-hidden"
            ></div>

            <h1 class="mb-4 text-4xl font-bold text-gray-900 dark:text-white md:text-5xl lg:text-6xl">
              {{ siteName }}
            </h1>
            <p class="mb-8 text-lg text-gray-600 dark:text-dark-300 md:text-xl">
              {{ siteSubtitle }}
            </p>

            <!-- CTA Button -->
            <div class="flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-primary px-8 py-3 text-base shadow-lg shadow-primary-500/30"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
              </router-link>
              <router-link
                to="/install-guide"
                class="btn btn-secondary px-6 py-3 text-base"
              >
                {{ t('home.installGuide') }}
              </router-link>
            </div>

            <!-- Feature Tags -->
            <div class="mt-8 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <div class="inline-flex items-center gap-2 rounded-full border border-gray-200/50 bg-white/80 px-4 py-2 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80">
                <Icon name="swap" size="sm" class="text-primary-500" />
                <span class="text-xs font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.subscriptionToApi') }}</span>
              </div>
              <div class="inline-flex items-center gap-2 rounded-full border border-gray-200/50 bg-white/80 px-4 py-2 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80">
                <Icon name="shield" size="sm" class="text-primary-500" />
                <span class="text-xs font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.stickySession') }}</span>
              </div>
              <div class="inline-flex items-center gap-2 rounded-full border border-gray-200/50 bg-white/80 px-4 py-2 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80">
                <Icon name="chart" size="sm" class="text-primary-500" />
                <span class="text-xs font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.realtimeBilling') }}</span>
              </div>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">terminal</span>
                </div>
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">curl</span>
                    <span class="code-flag">-X POST</span>
                    <span class="code-url">/v1/messages</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-comment"># Routing to upstream...</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-success">200 OK</span>
                    <span class="code-response">{ "content": "Hello!" }</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Content Sections -->
    <div class="relative z-10 px-4 sm:px-6">
      <div class="mx-auto max-w-6xl">
        <!-- Trust Logos -->
        <TrustLogos />

        <!-- Benefits -->
        <Benefits />

        <!-- Gallery (real subscription screenshots) -->
        <div v-if="homeGalleryEnabled" ref="galleryRef">
          <CircularGallery
            v-if="galleryData && galleryData.items.length > 0"
            :items="galleryData.items"
            :categories="galleryData.categories"
            :title="galleryData.title"
            :subtitle="galleryData.subtitle"
          />
        </div>

        <!-- How It Works -->
        <HowItWorks :video-config="videoConfig" />

        <!-- Pricing -->
        <Pricing />

        <!-- Testimonials -->
        <Testimonials :testimonials="homeTestimonials" />

        <!-- FAQ -->
        <Faq />

        <!-- Final CTA -->
        <FinalCta />
      </div>
    </div>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 dark:border-dark-800/50">
      <div class="mx-auto max-w-6xl px-6 py-10 sm:py-14">
        <!-- Top: multi-column -->
        <div class="grid gap-10 sm:grid-cols-2 lg:grid-cols-4">
          <!-- Brand -->
          <div class="sm:col-span-2 lg:col-span-2">
            <div class="mb-3 flex items-center gap-2">
              <img
                v-if="footerLogo"
                :src="footerLogo"
                :alt="siteName"
                class="h-7 w-7 rounded-lg object-contain"
              />
              <span class="text-base font-bold text-gray-900 dark:text-white">{{ siteName }}</span>
            </div>
            <p class="text-sm leading-relaxed text-gray-500 dark:text-dark-400">
              {{ siteSubtitle }}
            </p>
          </div>

          <!-- Product -->
          <div>
            <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('home.footer.product') }}
            </h4>
            <ul class="space-y-2 text-sm">
              <li>
                <router-link to="/install-guide" class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400">
                  {{ t('home.footer.installGuide') }}
                </router-link>
              </li>
              <li>
                <router-link to="/release-notes" class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400">
                  {{ t('home.footer.releaseNotes') }}
                </router-link>
              </li>
              <li v-if="docUrl">
                <a :href="docUrl" target="_blank" rel="noopener noreferrer" class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400">
                  {{ t('home.docs') }}
                </a>
              </li>
              <li>
                <a href="#faq" class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400">
                  {{ t('home.footer.faq') }}
                </a>
              </li>
            </ul>
          </div>

          <!-- Contact -->
          <div>
            <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('home.footer.contactUs') }}
            </h4>
            <ul class="space-y-2 text-sm">
              <li>
                <a
                  href="mailto:yian20133213@gmail.com"
                  class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
                >
                  yian20133213@gmail.com
                </a>
              </li>
              <li v-if="contactInfo && contactInfo !== 'yian20133213@gmail.com'">
                <a
                  :href="contactHref"
                  :target="contactIsLink ? '_blank' : undefined"
                  :rel="contactIsLink ? 'noopener noreferrer' : undefined"
                  class="text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400"
                >
                  {{ contactInfo }}
                </a>
              </li>
            </ul>
          </div>
        </div>

        <!-- Divider -->
        <div class="mt-8 border-t border-gray-200/50 pt-6 dark:border-dark-800/50">
          <p class="text-xs text-gray-400 dark:text-dark-500">
            &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
          </p>
        </div>
      </div>
    </footer>

    <!-- Floating Contact Button -->
    <Teleport to="body">
      <!-- Click-outside overlay to close popup -->
      <div
        v-if="showContactPopup"
        class="fixed inset-0 z-40"
        @click="showContactPopup = false"
      />
      <div class="fixed bottom-6 right-6 z-50">
        <!-- Popup -->
        <Transition name="contact-popup">
          <div
            v-if="showContactPopup"
            class="absolute bottom-16 right-0 w-72 rounded-2xl border border-gray-200 bg-white p-5 shadow-2xl dark:border-dark-700 dark:bg-dark-800"
          >
            <!-- Header -->
            <div class="mb-4 flex items-center justify-between">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('home.footer.contactUs') }}
              </h3>
              <button
                class="rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-white"
                @click="showContactPopup = false"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
            <!-- QR Codes -->
            <div class="flex justify-center gap-4">
              <div v-if="contactQRCodeWechat" class="text-center">
                <img :src="contactQRCodeWechat" :alt="t('home.footer.wechat')" class="h-28 w-28 rounded-lg border border-gray-100 dark:border-dark-700" />
                <p class="mt-2 text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('home.footer.wechat') }}</p>
              </div>
              <div v-if="contactQRCodeGroup" class="text-center">
                <img :src="contactQRCodeGroup" :alt="t('home.footer.group')" class="h-28 w-28 rounded-lg border border-gray-100 dark:border-dark-700" />
                <p class="mt-2 text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('home.footer.group') }}</p>
              </div>
            </div>
            <!-- Fallback: no QR codes configured -->
            <div v-if="!contactQRCodeWechat && !contactQRCodeGroup" class="text-center">
              <p class="text-sm text-gray-500 dark:text-dark-400">
                yian20133213@gmail.com
              </p>
            </div>
          </div>
        </Transition>

        <!-- FAB Button -->
        <button
          class="flex h-14 w-14 items-center justify-center rounded-full bg-primary-500 text-white shadow-lg shadow-primary-500/30 transition-all hover:bg-primary-600 hover:shadow-xl hover:shadow-primary-500/40 active:scale-95"
          @click="showContactPopup = !showContactPopup"
        >
          <svg class="h-6 w-6" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 1a9 9 0 00-9 9v7c0 1.66 1.34 3 3 3h3v-8H5v-2c0-3.87 3.13-7 7-7s7 3.13 7 7v2h-4v8h3c1.66 0 3-1.34 3-3v-7a9 9 0 00-9-9z" />
          </svg>
        </button>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, reactive, nextTick, defineAsyncComponent, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { useTheme } from '@/composables/useTheme'
import { preloadImages } from '@/utils/preload'
import { getHomeGallery } from '@/api/auth'
import type { GalleryData } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import StickyNav from '@/components/home/StickyNav.vue'
import TrustLogos from '@/components/home/TrustLogos.vue'
import Benefits from '@/components/home/Benefits.vue'
import HowItWorks from '@/components/home/HowItWorks.vue'
import Pricing from '@/components/home/Pricing.vue'
import Testimonials from '@/components/home/Testimonials.vue'
import Faq from '@/components/home/Faq.vue'
import FinalCta from '@/components/home/FinalCta.vue'

const CircularGallery = defineAsyncComponent(() =>
  import('@/components/home/CircularGallery.vue')
)

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()
const { isDark } = useTheme()

// Site settings
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Code80')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const homeTestimonials = computed(() => appStore.cachedPublicSettings?.home_testimonials || '')
const homeGalleryEnabled = computed(() => appStore.cachedPublicSettings?.home_gallery_enabled ?? false)

// Footer data
const footerLogo = computed(() => {
  const settings = appStore.cachedPublicSettings
  if (isDark.value && settings?.site_logo_dark) return settings.site_logo_dark
  return settings?.site_logo || ''
})
const contactInfo = computed(() => appStore.cachedPublicSettings?.contact_info || '')
const contactQRCodeWechat = computed(() => appStore.cachedPublicSettings?.contact_qrcode_wechat || '')
const contactQRCodeGroup = computed(() => appStore.cachedPublicSettings?.contact_qrcode_group || '')
const contactIsLink = computed(() => {
  const info = contactInfo.value
  return info.startsWith('http') || info.includes('@')
})
const contactHref = computed(() => {
  const info = contactInfo.value
  if (info.startsWith('http')) return info
  if (info.includes('@')) return `mailto:${info}`
  return '#'
})

// Gallery lazy loading
const galleryRef = ref<HTMLElement | null>(null)
const galleryData = ref<GalleryData | null>(null)
const galleryLoaded = ref(false)
let galleryObserver: IntersectionObserver | null = null

async function loadGalleryData() {
  if (galleryLoaded.value) return
  try {
    galleryData.value = await getHomeGallery()
    galleryLoaded.value = true // Only mark loaded on success
  } catch {
    // Transient failure — leave galleryLoaded false so observer can retry on next intersection
  }
}

// Set up IntersectionObserver when gallery becomes enabled (publicSettings loaded async)
watch(
  () => homeGalleryEnabled.value,
  async (enabled) => {
    if (!enabled) return
    await nextTick()
    if (!galleryRef.value || galleryLoaded.value) return
    galleryObserver?.disconnect()
    galleryObserver = new IntersectionObserver(
      async (entries) => {
        if (entries[0]?.isIntersecting) {
          await loadGalleryData()
          // Only disconnect if load succeeded (galleryLoaded becomes true)
          if (galleryLoaded.value) {
            galleryObserver?.disconnect()
          }
        }
      },
      { rootMargin: '200px' }
    )
    galleryObserver.observe(galleryRef.value)
  },
  { immediate: true }
)
const videoConfig = computed(() => {
  const raw = appStore.cachedPublicSettings?.install_guide_videos || ''
  if (!raw) return null
  try { return JSON.parse(raw) } catch { return null }
})

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Sanitize HTML content to prevent XSS attacks
const sanitizedHomeContent = computed(() => {
  if (!homeContent.value || isHomeContentUrl.value) return ''
  const temp = document.createElement('div')
  temp.innerHTML = homeContent.value
  const scripts = temp.querySelectorAll('script')
  scripts.forEach(s => s.remove())
  const allElements = temp.querySelectorAll('*')
  allElements.forEach(el => {
    Array.from(el.attributes).forEach(attr => {
      if (attr.name.startsWith('on')) el.removeAttribute(attr.name)
    })
    if (el.hasAttribute('href')) {
      const href = el.getAttribute('href') || ''
      if (href.toLowerCase().startsWith('javascript:') || href.toLowerCase().startsWith('data:'))
        el.removeAttribute('href')
    }
    if (el.hasAttribute('src')) {
      const src = el.getAttribute('src') || ''
      if (src.toLowerCase().startsWith('javascript:')) el.removeAttribute('src')
    }
  })
  return temp.innerHTML
})

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Contact popup
const showContactPopup = ref(false)

// ==================== Opening Animation ====================
const showAnimation = ref(true)
const showHeroContent = ref(false)
const carRef = ref<HTMLImageElement | HTMLImageElement[] | null>(null)
const siteLogoRef = ref<HTMLElement | null>(null)
const stickyNavRef = ref<InstanceType<typeof StickyNav> | null>(null)
const animationStarted = ref(false)

// LLM Logo list
const llmLogos = [
  { name: 'Claude', src: '/llmLogo/sm/Claude.png' },
  { name: 'ChatGPT', src: '/llmLogo/sm/ChatGPT.png' },
  { name: 'Gemini', src: '/llmLogo/sm/Gemini.jpg' },
  { name: 'Antigravity', src: '/llmLogo/sm/Antigravity.jpg' }
]

// Car position state
const carPosition = reactive({ x: -150, y: 0, scale: 1, opacity: 1 })

const carStyle = computed(() => ({
  transform: `translateX(${carPosition.x}px) scale(${carPosition.scale})`,
  opacity: carPosition.opacity,
  top: `${carPosition.y}px`,
  left: '0'
}))

// Falling logos state
interface FallingLogo {
  name: string
  src: string
  style: { transform: string; opacity: number; left: string; top: string; transition: string }
}

const fallingLogos = ref<FallingLogo[]>([])

// Audio for car animation
const busDrivingSound = ref<HTMLAudioElement | null>(null)
const busHornSound = ref<HTMLAudioElement | null>(null)

// Animation configuration constants
const ANIMATION_CONFIG = {
  PRELOAD_DELAY: 150,
  PRELOAD_MAX_WAIT: 800,
  PHASE1_DURATION: 2200,
  PHASE1_PAUSE: 200,
  PHASE2_DURATION: 600,
  LOGO_FLY_DURATION: 800,
  LOGO_FADE_START: 0.4,
  LOGO_DROP_DELAY: 30,
  HORN_SOUND_DELAY: 200,
  ANIMATION_END_DELAY: 200,
  HEADER_SCROLL_OFFSET: 96,
  MIN_TOP_OFFSET: 8,
  MIN_EDGE_OFFSET: 12,
  CAR_STOP_POSITION: 0.55,
  BOUNCING_CYCLES: 4,
  BOUNCE_AMPLITUDE: 2
} as const

const DEFAULT_CAR_HEIGHT = 80
const DEFAULT_CAR_WIDTH = 160

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

function getCarMetrics() {
  const carEl = Array.isArray(carRef.value)
    ? carRef.value.find(el => el?.naturalWidth) ?? carRef.value[0]
    : carRef.value
  const height = DEFAULT_CAR_HEIGHT
  const naturalWidth = carEl?.naturalWidth ?? 0
  const naturalHeight = carEl?.naturalHeight ?? 0
  const width = naturalWidth && naturalHeight
    ? (naturalWidth / naturalHeight) * height
    : carEl?.getBoundingClientRect().width || DEFAULT_CAR_WIDTH
  return { width, height }
}

function preloadAnimationAssets(): Promise<void> {
  const supportsWebP = document.createElement('canvas').toDataURL('image/webp').startsWith('data:image/webp')
  const carImages = supportsWebP
    ? ['/car.webp', '/car_night.webp']
    : ['/car.png', '/car_night.png']
  const imagesToPreload = [...carImages, ...llmLogos.map(logo => logo.src)]
  return preloadImages(imagesToPreload)
}

function initAudio() {
  busDrivingSound.value = new Audio('/audio/bus-driving.MP3')
  busDrivingSound.value.volume = 0.5
  busHornSound.value = new Audio('/audio/bus-horn.MP3')
  busHornSound.value.volume = 0.6
}

function playHorn() {
  if (!busHornSound.value) {
    busHornSound.value = new Audio('/audio/bus-horn.MP3')
    busHornSound.value.volume = 0.6
  }
  busHornSound.value.currentTime = 0
  busHornSound.value.play().catch(() => {})
}

function startAnimation() {
  if (animationStarted.value) return
  animationStarted.value = true
  const screenWidth = window.innerWidth
  const screenHeight = window.innerHeight

  const { width: carWidth, height: carHeight } = getCarMetrics()
  const minY = 0
  const maxY = Math.max(0, screenHeight - carHeight - ANIMATION_CONFIG.MIN_TOP_OFFSET)
  const maxX = Math.max(0, screenWidth - carWidth - ANIMATION_CONFIG.MIN_EDGE_OFFSET)

  initAudio()
  if (busDrivingSound.value) {
    busDrivingSound.value.play().catch(() => {})
  }

  // Car drives along the very top of the screen (flush with viewport top)
  const headerY = 0
  // Car final destination: the StickyNav logo area (top-left)
  let logoTargetX = 24
  let logoTargetY = 0

  // Find the StickyNav logo position for the car to shrink into
  const navLogoEl = document.querySelector('.sticky.top-0 img')
  if (navLogoEl) {
    const rect = navLogoEl.getBoundingClientRect()
    logoTargetX = rect.left
    logoTargetY = rect.top
  }

  const startX = -(carWidth + ANIMATION_CONFIG.MIN_EDGE_OFFSET)
  const midX = Math.min(screenWidth * ANIMATION_CONFIG.CAR_STOP_POSITION, maxX)

  carPosition.x = startX
  carPosition.y = headerY

  const phase1Duration = ANIMATION_CONFIG.PHASE1_DURATION
  const phase1Start = Date.now()

  const dropPositions = [0.2, 0.35, 0.5, 0.65, 0.8]
  let droppedCount = 0

  function animatePhase1() {
    const elapsed = Date.now() - phase1Start
    const progress = Math.min(elapsed / phase1Duration, 1)
    const eased = 1 - Math.pow(1 - progress, 2)

    carPosition.x = startX + (midX - startX) * eased
    const bounceY = headerY + Math.sin(progress * Math.PI * ANIMATION_CONFIG.BOUNCING_CYCLES) * ANIMATION_CONFIG.BOUNCE_AMPLITUDE
    carPosition.y = clamp(bounceY, minY, maxY)

    while (droppedCount < dropPositions.length && progress >= dropPositions[droppedCount]) {
      dropSingleLogo(droppedCount)
      droppedCount++
    }

    if (progress < 1) {
      requestAnimationFrame(animatePhase1)
    } else {
      setTimeout(() => {
        // Re-read nav logo position for phase 2
        const navLogo = document.querySelector('.sticky.top-0 img')
        if (navLogo) {
          const rect = navLogo.getBoundingClientRect()
          logoTargetX = rect.left
          logoTargetY = rect.top
        }
        logoTargetX = clamp(logoTargetX, 0, maxX)
        logoTargetY = clamp(logoTargetY, minY, maxY)
        moveCarToLogo(logoTargetX, logoTargetY)
      }, ANIMATION_CONFIG.PHASE1_PAUSE)
    }
  }

  animatePhase1()

  function dropSingleLogo(index: number) {
    const logo = llmLogos[index]
    if (!logo) return

    const logoStartX = carPosition.x + 50
    const logoStartY = carPosition.y + 50

    // Logos fall down to the TrustLogos section
    const trustLogosEl = document.getElementById('trust-logos')
    let targetLogoY = screenHeight - 50
    let targetLogoCenterX = screenWidth / 2
    if (trustLogosEl) {
      const rect = trustLogosEl.getBoundingClientRect()
      targetLogoY = rect.top + rect.height / 2
      // Target the <img> inside each logo link, not the whole <a> (which includes text)
      const logoImages = trustLogosEl.querySelectorAll('img')
      if (logoImages[index]) {
        const imgRect = logoImages[index].getBoundingClientRect()
        // Use image center, then offset by half the falling logo size (40px / 2 = 20) to land centered
        targetLogoCenterX = imgRect.left + imgRect.width / 2 - 20
        targetLogoY = imgRect.top + imgRect.height / 2 - 20
      }
    }
    const targetLogoX = targetLogoCenterX

    const fallingLogo: FallingLogo = {
      name: logo.name,
      src: logo.src,
      style: {
        transform: `translate(0px, 0px) scale(1) rotate(0deg)`,
        opacity: 1,
        left: `${logoStartX}px`,
        top: `${logoStartY}px`,
        transition: 'none'
      }
    }

    fallingLogos.value.push(fallingLogo)
    const logoIndex = fallingLogos.value.length - 1

    nextTick(() => {
      const deltaX = targetLogoX - logoStartX
      const deltaY = targetLogoY - logoStartY
      const flyDuration = ANIMATION_CONFIG.LOGO_FLY_DURATION
      const rotation = (Math.random() - 0.5) * 360

      setTimeout(() => {
        if (fallingLogos.value[logoIndex]) {
          fallingLogos.value[logoIndex].style = {
            ...fallingLogos.value[logoIndex].style,
            transform: `translate(${deltaX}px, ${deltaY}px) scale(0.6) rotate(${rotation}deg)`,
            opacity: 0,
            transition: `transform ${flyDuration}ms cubic-bezier(0.25, 0.46, 0.45, 0.94), opacity ${flyDuration * ANIMATION_CONFIG.LOGO_FADE_START}ms ease-out ${flyDuration * (1 - ANIMATION_CONFIG.LOGO_FADE_START)}ms`
          }
        }
      }, ANIMATION_CONFIG.LOGO_DROP_DELAY)
    })
  }

  function moveCarToLogo(targetX: number, targetY: number) {
    const phase2Duration = ANIMATION_CONFIG.PHASE2_DURATION
    const phase2Start = Date.now()
    const carStartX = carPosition.x
    const carStartY = carPosition.y

    function animatePhase2() {
      const elapsed = Date.now() - phase2Start
      const progress = Math.min(elapsed / phase2Duration, 1)
      const eased = progress < 0.5 ? 4 * progress * progress * progress : 1 - Math.pow(-2 * progress + 2, 3) / 2

      carPosition.x = carStartX + (targetX - carStartX) * eased
      carPosition.y = carStartY + (targetY - carStartY) * eased
      carPosition.scale = 1 - eased * 0.6
      carPosition.opacity = 1 - eased

      if (progress < 1) {
        requestAnimationFrame(animatePhase2)
      } else {
        playHorn()
        setTimeout(() => {
          if (animationSafetyTimer) {
            window.clearTimeout(animationSafetyTimer)
            animationSafetyTimer = null
          }
          showAnimation.value = false
          if (scrollCleanup) {
            scrollCleanup()
            scrollCleanup = null
          }
        }, ANIMATION_CONFIG.ANIMATION_END_DELAY)
      }
    }

    animatePhase2()
  }
}

function shouldSkipAnimation(): boolean {
  const prefersReducedMotion =
    typeof window !== 'undefined' &&
    window.matchMedia &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const connection = (navigator as Navigator & {
    connection?: { effectiveType?: string; saveData?: boolean }
  }).connection
  const saveData = Boolean(connection?.saveData)
  const effectiveType = connection?.effectiveType || ''
  const slowConnection = ['slow-2g', '2g', '3g'].includes(effectiveType)
  return prefersReducedMotion || saveData || slowConnection
}

function revealContentImmediately() {
  showHeroContent.value = true
}

let animationSafetyTimer: number | null = null
let scrollCleanup: (() => void) | null = null

function endAnimationEarly() {
  if (!showAnimation.value) return
  if (animationSafetyTimer) {
    window.clearTimeout(animationSafetyTimer)
    animationSafetyTimer = null
  }
  if (busDrivingSound.value) {
    busDrivingSound.value.pause()
  }
  showAnimation.value = false
  if (scrollCleanup) {
    scrollCleanup()
    scrollCleanup = null
  }
}

async function initAnimation() {
  if (shouldSkipAnimation()) {
    revealContentImmediately()
    showAnimation.value = false
    return
  }

  // Show hero content immediately so users can scroll and interact during animation
  revealContentImmediately()

  // 用户滚动时快速结束动画，避免 fixed overlay 与滚动内容坐标错位
  const onScroll = () => endAnimationEarly()
  window.addEventListener('scroll', onScroll, { once: true, passive: true })
  scrollCleanup = () => window.removeEventListener('scroll', onScroll)

  animationSafetyTimer = window.setTimeout(() => {
    showAnimation.value = false
    if (scrollCleanup) {
      scrollCleanup()
      scrollCleanup = null
    }
  }, 12000)

  const fallbackTimer = window.setTimeout(() => {
    startAnimation()
  }, ANIMATION_CONFIG.PRELOAD_MAX_WAIT)

  preloadAnimationAssets()
    .catch((error) => {
      console.error('预加载资源失败:', error)
    })
    .finally(() => {
      window.clearTimeout(fallbackTimer)
      setTimeout(() => {
        startAnimation()
      }, ANIMATION_CONFIG.PRELOAD_DELAY)
    })
}

onMounted(async () => {
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  initAnimation()

  // Gallery observer will be set up by the watcher below
})

onUnmounted(() => {
  if (scrollCleanup) {
    scrollCleanup()
    scrollCleanup = null
  }
  if (animationSafetyTimer) {
    window.clearTimeout(animationSafetyTimer)
    animationSafetyTimer = null
  }
  if (busDrivingSound.value) {
    busDrivingSound.value.pause()
    busDrivingSound.value.src = ''
    busDrivingSound.value = null
  }
  if (busHornSound.value) {
    busHornSound.value.pause()
    busHornSound.value.src = ''
    busHornSound.value = null
  }
  galleryObserver?.disconnect()
})
</script>

<style scoped>
/* Car Animation */
.car-animation {
  will-change: transform, opacity;
  filter: drop-shadow(0 8px 16px rgba(0, 0, 0, 0.3));
}

/* Terminal Container */
.terminal-container {
  position: relative;
  display: inline-block;
}

/* Terminal Window */
.terminal-window {
  width: 420px;
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

.btn-close { background: #ef4444; }
.btn-minimize { background: #eab308; }
.btn-maximize { background: #22c55e; }

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

.line-1 { animation-delay: 0.3s; }
.line-2 { animation-delay: 1s; }
.line-3 { animation-delay: 1.8s; }
.line-4 { animation-delay: 2.5s; }

@keyframes line-appear {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}

.code-prompt { color: #22c55e; font-weight: bold; }
.code-cmd { color: #38bdf8; }
.code-flag { color: #a78bfa; }
.code-url { color: #d97757; }
.code-comment { color: #64748b; font-style: italic; }
.code-success {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response { color: #fbbf24; }

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(217, 119, 87, 0.2),
    0 0 40px rgba(217, 119, 87, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

/* Contact popup transition */
.contact-popup-enter-active,
.contact-popup-leave-active {
  transition: all 0.2s ease;
}
.contact-popup-enter-from,
.contact-popup-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.95);
}
</style>
