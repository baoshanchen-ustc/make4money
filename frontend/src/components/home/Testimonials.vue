<template>
  <section v-if="parsedTestimonials.length" class="py-12 sm:py-16">
    <h2 class="mb-3 text-center text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
      {{ t('home.testimonials.title') }}
    </h2>
    <p class="mb-8 text-center text-sm text-gray-500 dark:text-dark-400 sm:mb-12 sm:text-base">
      {{ t('home.testimonials.subtitle') }}
    </p>

    <!-- Marquee container -->
    <div class="testimonials-marquee" @mouseenter="paused = true" @mouseleave="paused = false">
      <div class="marquee-track" :class="{ paused }">
        <div
          v-for="(item, i) in doubledTestimonials"
          :key="i"
          class="marquee-card rounded-2xl border border-gray-200/50 bg-white/60 p-5 backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/60"
        >
          <!-- Stars -->
          <div class="mb-3 flex gap-0.5">
            <svg v-for="s in 5" :key="s" class="h-4 w-4" :class="s <= (item.rating || 5) ? 'text-amber-400' : 'text-gray-200 dark:text-dark-600'" fill="currentColor" viewBox="0 0 20 20">
              <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
            </svg>
          </div>

          <!-- Content -->
          <p class="mb-4 text-sm leading-relaxed text-gray-600 dark:text-dark-300">{{ item.content }}</p>

          <!-- Author -->
          <div class="flex items-center gap-3">
            <div class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-semibold text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
              {{ (item.name || '?')[0] }}
            </div>
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ item.name }}</p>
              <p v-if="item.role" class="text-xs text-gray-500 dark:text-dark-400">{{ item.role }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const paused = ref(false)

const props = defineProps<{
  testimonials: string
}>()

interface Testimonial {
  name: string
  role?: string
  content: string
  rating?: number
}

const parsedTestimonials = computed<Testimonial[]>(() => {
  if (!props.testimonials) return []
  try {
    const arr = JSON.parse(props.testimonials)
    if (!Array.isArray(arr)) return []
    return arr.filter((item: any) => item && item.name && item.content)
  } catch {
    return []
  }
})

// 复制一份实现无缝循环
const doubledTestimonials = computed(() => [...parsedTestimonials.value, ...parsedTestimonials.value])
</script>

<style scoped>
.testimonials-marquee {
  overflow: hidden;
  mask-image: linear-gradient(to right, transparent, black 5%, black 95%, transparent);
  -webkit-mask-image: linear-gradient(to right, transparent, black 5%, black 95%, transparent);
}

.marquee-track {
  display: flex;
  gap: 1.5rem;
  width: max-content;
  animation: marquee-scroll 40s linear infinite;
}

.marquee-track.paused {
  animation-play-state: paused;
}

.marquee-card {
  flex-shrink: 0;
  width: 320px;
}

@keyframes marquee-scroll {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .marquee-track {
    animation: none;
  }
}
</style>
