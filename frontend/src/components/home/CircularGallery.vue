<template>
  <section class="py-12 sm:py-16">
    <h2 v-if="title" class="mb-3 text-center text-2xl font-bold text-gray-900 dark:text-white sm:text-3xl">
      {{ title }}
    </h2>
    <p v-if="subtitle" class="mb-6 text-center text-sm text-gray-500 dark:text-dark-400 sm:mb-8 sm:text-base">
      {{ subtitle }}
    </p>

    <!-- Category Tabs -->
    <div v-if="categories.length > 1" class="mb-6 flex flex-wrap justify-center gap-2">
      <button
        class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
        :class="activeCategory === null
          ? 'bg-primary-500 text-white shadow-sm'
          : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-300 dark:hover:bg-dark-600'"
        @click="activeCategory = null"
      >
        {{ t('home.gallery.all') }}
      </button>
      <button
        v-for="cat in categories"
        :key="cat.key"
        class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
        :class="activeCategory === cat.key
          ? 'bg-primary-500 text-white shadow-sm'
          : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-dark-300 dark:hover:bg-dark-600'"
        @click="activeCategory = cat.key"
      >
        {{ cat.label }}
      </button>
    </div>

    <!-- Gallery Container -->
    <div
      v-if="filteredItems.length > 0"
      ref="containerRef"
      class="gallery-container relative mx-auto overflow-hidden rounded-xl"
      :style="{ height: galleryHeight + 'px' }"
    />

    <!-- Empty State -->
    <div v-else class="py-12 text-center text-gray-400 dark:text-dark-500">
      {{ t('home.gallery.noImages') }}
    </div>

    <!-- Lightbox -->
    <Teleport to="body">
      <Transition name="lightbox">
        <div
          v-if="lightboxImage"
          class="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 backdrop-blur-sm"
          @click="lightboxImage = null"
        >
          <button
            class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/10 text-2xl text-white/70 transition hover:bg-white/20 hover:text-white"
            @click.stop="lightboxImage = null"
          >
            &times;
          </button>
          <div class="flex max-h-[90vh] max-w-[90vw] flex-col items-center" @click.stop>
            <img
              :src="lightboxImage"
              class="max-h-[85vh] max-w-[90vw] rounded-lg object-contain shadow-2xl"
              draggable="false"
            />
            <p v-if="lightboxTitle" class="mt-3 text-sm text-white/80">{{ lightboxTitle }}</p>
          </div>
        </div>
      </Transition>
    </Teleport>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTheme } from '@/composables/useTheme'
import type { GalleryItem, GalleryCategory } from '@/types'
import { Renderer, Camera, Transform, Plane, Mesh, Program, Texture } from 'ogl'

const { t } = useI18n()
const { isDark } = useTheme()

const props = defineProps<{
  items: GalleryItem[]
  categories: GalleryCategory[]
  title?: string
  subtitle?: string
}>()

const containerRef = ref<HTMLElement | null>(null)
const activeCategory = ref<string | null>(null)
const galleryHeight = ref(400)
const lightboxImage = ref<string | null>(null)
const lightboxTitle = ref<string | null>(null)

const filteredItems = computed(() => {
  if (!activeCategory.value) return props.items
  return props.items.filter(item => item.category === activeCategory.value)
})

const textColor = computed(() => isDark.value ? '#d1d5db' : '#545050')

const prefersReducedMotion = typeof window !== 'undefined'
  && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

// ─── Utilities ───

function lerp(p1: number, p2: number, t: number): number {
  return p1 + (p2 - p1) * t
}

function debounce<T extends (...args: any[]) => void>(func: T, wait: number): T {
  let timeout: ReturnType<typeof setTimeout> | null = null
  return function (this: any, ...args: any[]) {
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(() => func.apply(this, args), wait)
  } as T
}

function isValidImageSrc(src: string): boolean {
  return src.startsWith('data:image/') || src.startsWith('http://') || src.startsWith('https://')
}

function createTextTexture(gl: any, text: string, font: string, color: string) {
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  if (!ctx) return { texture: new Texture(gl, { generateMipmaps: false }), width: 100, height: 30 }
  ctx.font = font
  const metrics = ctx.measureText(text)
  const textWidth = Math.ceil(metrics.width)
  const textHeight = Math.ceil(parseInt(font, 10) * 1.2)
  canvas.width = textWidth + 20
  canvas.height = textHeight + 20
  ctx.font = font
  ctx.fillStyle = color
  ctx.textBaseline = 'middle'
  ctx.textAlign = 'center'
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  ctx.fillText(text, canvas.width / 2, canvas.height / 2)
  const texture = new Texture(gl, { generateMipmaps: false })
  texture.image = canvas
  return { texture, width: canvas.width, height: canvas.height }
}

// ─── Title class ───

class TitleLabel {
  mesh: Mesh
  constructor(gl: any, parentPlane: Mesh, text: string, color: string, font: string) {
    const { texture, width, height } = createTextTexture(gl, text, font, color)
    const geometry = new Plane(gl)
    const program = new Program(gl, {
      vertex: `
        attribute vec3 position;
        attribute vec2 uv;
        uniform mat4 modelViewMatrix;
        uniform mat4 projectionMatrix;
        varying vec2 vUv;
        void main() {
          vUv = uv;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragment: `
        precision highp float;
        uniform sampler2D tMap;
        varying vec2 vUv;
        void main() {
          vec4 color = texture2D(tMap, vUv);
          if (color.a < 0.1) discard;
          gl_FragColor = color;
        }
      `,
      uniforms: { tMap: { value: texture } },
      transparent: true,
    })
    this.mesh = new Mesh(gl, { geometry, program })
    const aspect = width / height
    const textH = parentPlane.scale.y * 0.15
    const textW = textH * aspect
    this.mesh.scale.set(textW, textH, 1)
    this.mesh.position.y = -parentPlane.scale.y * 0.5 - textH * 0.5 - 0.05
    this.mesh.setParent(parentPlane)
  }
}

// ─── Media class ───

const VERTEX_SHADER = `
  precision highp float;
  attribute vec3 position;
  attribute vec2 uv;
  uniform mat4 modelViewMatrix;
  uniform mat4 projectionMatrix;
  uniform float uTime;
  uniform float uSpeed;
  varying vec2 vUv;
  void main() {
    vUv = uv;
    vec3 p = position;
    p.z = (sin(p.x * 4.0 + uTime) * 1.5 + cos(p.y * 2.0 + uTime) * 1.5) * (0.1 + uSpeed * 0.5);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(p, 1.0);
  }
`

const FRAGMENT_SHADER = `
  precision highp float;
  uniform vec2 uImageSizes;
  uniform vec2 uPlaneSizes;
  uniform sampler2D tMap;
  uniform float uBorderRadius;
  varying vec2 vUv;

  float roundedBoxSDF(vec2 p, vec2 b, float r) {
    vec2 d = abs(p) - b;
    return length(max(d, vec2(0.0))) + min(max(d.x, d.y), 0.0) - r;
  }

  void main() {
    vec2 ratio = vec2(
      min((uPlaneSizes.x / uPlaneSizes.y) / (uImageSizes.x / uImageSizes.y), 1.0),
      min((uPlaneSizes.y / uPlaneSizes.x) / (uImageSizes.y / uImageSizes.x), 1.0)
    );
    vec2 uv = vec2(
      vUv.x * ratio.x + (1.0 - ratio.x) * 0.5,
      vUv.y * ratio.y + (1.0 - ratio.y) * 0.5
    );
    vec4 color = texture2D(tMap, uv);

    float d = roundedBoxSDF(vUv - 0.5, vec2(0.5 - uBorderRadius), uBorderRadius);
    float edgeSmooth = 0.002;
    float alpha = 1.0 - smoothstep(-edgeSmooth, edgeSmooth, d);

    gl_FragColor = vec4(color.rgb, alpha);
  }
`

// Plane dimension bases (at 1500px screen height)
const PLANE_W = 900
const PLANE_H = 600
const HOVER_SCALE = 1.15

interface ScrollState {
  ease: number
  current: number
  target: number
  last: number
  position: number
}

interface ScreenSize { width: number; height: number }

class MediaItem {
  plane!: Mesh
  program!: Program
  title!: TitleLabel
  x = 0
  width = 0
  widthTotal = 0
  extra = 0
  isBefore = false
  isAfter = false
  baseScaleX = 0
  baseScaleY = 0
  hoverTarget = 1.0
  hoverCurrent = 1.0
  private scale = 1
  private screen: ScreenSize
  private viewport: ScreenSize

  constructor(
    private gl: any,
    private geometry: Plane,
    readonly imageSrc: string,
    readonly text: string,
    private index: number,
    private length: number,
    readonly originalIndex: number,
    _renderer: Renderer,
    private scene: Transform,
    screen: ScreenSize,
    viewport: ScreenSize,
    private bend: number,
    private textColorVal: string,
    private borderRadius: number,
    private font: string,
  ) {
    this.screen = { ...screen }
    this.viewport = { ...viewport }
    this.createShader()
    this.createMesh()
    this.createTitle()
    this.onResize()
  }

  createShader() {
    const texture = new Texture(this.gl, { generateMipmaps: true })
    this.program = new Program(this.gl, {
      depthTest: false,
      depthWrite: false,
      vertex: VERTEX_SHADER,
      fragment: FRAGMENT_SHADER,
      uniforms: {
        tMap: { value: texture },
        uPlaneSizes: { value: [0, 0] },
        uImageSizes: { value: [0, 0] },
        uSpeed: { value: 0 },
        uTime: { value: 100 * Math.random() },
        uBorderRadius: { value: this.borderRadius },
      },
      transparent: true,
    })
    if (isValidImageSrc(this.imageSrc)) {
      const img = new Image()
      img.crossOrigin = 'anonymous'
      img.src = this.imageSrc
      img.onload = () => {
        texture.image = img
        ;(this.program as any).uniforms.uImageSizes.value = [img.naturalWidth, img.naturalHeight]
      }
    }
  }

  createMesh() {
    this.plane = new Mesh(this.gl, {
      geometry: this.geometry,
      program: this.program,
    })
    this.plane.setParent(this.scene)
  }

  createTitle() {
    this.title = new TitleLabel(this.gl, this.plane, this.text, this.textColorVal, this.font)
  }

  update(scroll: ScrollState, direction: string) {
    this.plane.position.x = this.x - scroll.current - this.extra

    const x = this.plane.position.x
    const H = this.viewport.width / 2

    if (this.bend === 0) {
      this.plane.position.y = 0
      this.plane.rotation.z = 0
    } else {
      const B_abs = Math.abs(this.bend)
      const R = (H * H + B_abs * B_abs) / (2 * B_abs)
      const effectiveX = Math.min(Math.abs(x), H)
      const arc = R - Math.sqrt(R * R - effectiveX * effectiveX)

      if (this.bend > 0) {
        this.plane.position.y = -arc
        this.plane.rotation.z = -Math.sign(x) * Math.asin(effectiveX / R)
      } else {
        this.plane.position.y = arc
        this.plane.rotation.z = Math.sign(x) * Math.asin(effectiveX / R)
      }
    }

    // Smooth hover scale
    this.hoverCurrent = lerp(this.hoverCurrent, this.hoverTarget, 0.1)
    this.plane.scale.x = this.baseScaleX * this.hoverCurrent
    this.plane.scale.y = this.baseScaleY * this.hoverCurrent

    const speed = scroll.current - scroll.last
    ;(this.program as any).uniforms.uTime.value += 0.04
    ;(this.program as any).uniforms.uSpeed.value = speed

    const planeOffset = this.baseScaleX / 2
    const viewportOffset = this.viewport.width / 2
    this.isBefore = this.plane.position.x + planeOffset < -viewportOffset
    this.isAfter = this.plane.position.x - planeOffset > viewportOffset

    if (direction === 'right' && this.isBefore) {
      this.extra -= this.widthTotal
      this.isBefore = this.isAfter = false
    }
    if (direction === 'left' && this.isAfter) {
      this.extra += this.widthTotal
      this.isBefore = this.isAfter = false
    }
  }

  onResize(params?: { screen?: ScreenSize; viewport?: ScreenSize }) {
    if (params?.screen) this.screen = params.screen
    if (params?.viewport) this.viewport = params.viewport

    this.scale = this.screen.height / 1500
    this.baseScaleY = (this.viewport.height * (PLANE_H * this.scale)) / this.screen.height
    this.baseScaleX = (this.viewport.width * (PLANE_W * this.scale)) / this.screen.width
    this.plane.scale.x = this.baseScaleX
    this.plane.scale.y = this.baseScaleY
    ;(this.program as any).uniforms.uPlaneSizes.value = [this.baseScaleX, this.baseScaleY]

    this.width = this.baseScaleX + 2
    this.widthTotal = this.width * this.length
    this.x = this.width * this.index
  }
}

// ─── Gallery App ───

class GalleryApp {
  private renderer!: Renderer
  private gl!: any
  private camera!: Camera
  private scene!: Transform
  private planeGeometry!: Plane
  private medias: MediaItem[] = []
  private screen: ScreenSize = { width: 0, height: 0 }
  private viewport: ScreenSize = { width: 0, height: 0 }
  private scroll: ScrollState = { ease: 0.05, current: 0, target: 0, last: 0, position: 0 }
  private isDown = false
  private hasDragged = false
  private start = 0
  private raf = 0
  private autoScrollSpeed = 0.15
  private autoScrollTimer: ReturnType<typeof setTimeout> | null = null
  private isInteracting = false
  private hoveredIndex = -1
  private mouseX = -1
  private mouseY = -1
  private onCheckDebounce: () => void

  private boundOnResize: () => void
  private boundOnWheel: (e: WheelEvent) => void
  private boundOnTouchDown: (e: MouseEvent | TouchEvent) => void
  private boundOnTouchMove: (e: MouseEvent | TouchEvent) => void
  private boundOnTouchUp: () => void
  private boundOnMouseMove: (e: MouseEvent) => void
  private boundOnMouseLeave: () => void
  private boundUpdate: () => void

  constructor(
    private container: HTMLElement,
    items: Array<{ image: string; text: string }>,
    private bend: number,
    textColorVal: string,
    borderRadius: number,
    font: string,
    private scrollSpeed: number,
    scrollEase: number,
    private reducedMotion: boolean,
    private onItemClick?: (item: { image: string; text: string }) => void,
  ) {
    this.scroll.ease = scrollEase
    this.onCheckDebounce = debounce(this.onCheck.bind(this), 200)
    this.boundOnResize = this.onResize.bind(this)
    this.boundOnWheel = this.onWheel.bind(this)
    this.boundOnTouchDown = this.onTouchDown.bind(this)
    this.boundOnTouchMove = this.onTouchMove.bind(this)
    this.boundOnTouchUp = this.onTouchUp.bind(this)
    this.boundOnMouseMove = this.onMouseMove.bind(this)
    this.boundOnMouseLeave = this.onMouseLeave.bind(this)
    this.boundUpdate = this.update.bind(this)

    this.createRenderer()
    this.createCamera()
    this.createScene()
    this.onResize()
    this.createGeometry()
    this.createMedias(items, textColorVal, borderRadius, font)
    this.addEventListeners()
    this.update()
  }

  private createRenderer() {
    this.renderer = new Renderer({
      alpha: true,
      antialias: true,
      dpr: Math.min(window.devicePixelRatio || 1, 2),
    })
    this.gl = this.renderer.gl
    this.gl.clearColor(0, 0, 0, 0)
    this.container.appendChild(this.gl.canvas)
  }

  private createCamera() {
    this.camera = new Camera(this.gl)
    ;(this.camera as any).fov = 45
    this.camera.position.z = 20
  }

  private createScene() {
    this.scene = new Transform()
  }

  private createGeometry() {
    this.planeGeometry = new Plane(this.gl, {
      heightSegments: 50,
      widthSegments: 100,
    })
  }

  private createMedias(
    items: Array<{ image: string; text: string }>,
    textColorVal: string,
    borderRadius: number,
    font: string,
  ) {
    const doubled = items.concat(items)
    this.medias = doubled.map((data, index) => {
      return new MediaItem(
        this.gl,
        this.planeGeometry,
        data.image,
        data.text,
        index,
        doubled.length,
        index % items.length,
        this.renderer,
        this.scene,
        this.screen,
        this.viewport,
        this.bend,
        textColorVal,
        borderRadius,
        font,
      )
    })
  }

  private hitTest() {
    if (this.mouseX < 0 || this.mouseY < 0 || !this.medias.length) {
      this.setHovered(-1)
      return
    }

    // Convert mouse to viewport coords
    const mx = ((this.mouseX / this.screen.width) * 2 - 1) * this.viewport.width / 2
    const my = -((this.mouseY / this.screen.height) * 2 - 1) * this.viewport.height / 2

    let closest = -1
    let closestDist = Infinity

    for (let i = 0; i < this.medias.length; i++) {
      const m = this.medias[i]
      const dx = mx - m.plane.position.x
      const dy = my - m.plane.position.y
      const halfW = m.baseScaleX * 0.5
      const halfH = m.baseScaleY * 0.5

      if (Math.abs(dx) < halfW && Math.abs(dy) < halfH) {
        const dist = dx * dx + dy * dy
        if (dist < closestDist) {
          closestDist = dist
          closest = i
        }
      }
    }

    this.setHovered(closest)
  }

  private setHovered(index: number) {
    if (this.hoveredIndex === index) return
    this.hoveredIndex = index

    for (let i = 0; i < this.medias.length; i++) {
      this.medias[i].hoverTarget = (i === index) ? HOVER_SCALE : 1.0
    }

    this.container.style.cursor = index >= 0
      ? 'pointer'
      : (this.isDown ? 'grabbing' : 'grab')
  }

  private onMouseMove(e: MouseEvent) {
    const rect = this.container.getBoundingClientRect()
    this.mouseX = e.clientX - rect.left
    this.mouseY = e.clientY - rect.top
  }

  private onMouseLeave() {
    this.mouseX = -1
    this.mouseY = -1
    this.setHovered(-1)
  }

  private onTouchDown(e: MouseEvent | TouchEvent) {
    this.isDown = true
    this.hasDragged = false
    this.isInteracting = true
    this.autoScrollSpeed = 0
    this.scroll.position = this.scroll.current
    this.start = 'touches' in e ? e.touches[0].clientX : e.clientX
  }

  private onTouchMove(e: MouseEvent | TouchEvent) {
    if (!this.isDown) return
    const x = 'touches' in e ? e.touches[0].clientX : e.clientX
    if (Math.abs(x - this.start) > 5) this.hasDragged = true
    const distance = (this.start - x) * (this.scrollSpeed * 0.025)
    this.scroll.target = this.scroll.position + distance
  }

  private onTouchUp() {
    if (!this.isDown) return
    this.isDown = false

    // Click (not drag) on a hovered item -> open lightbox
    if (!this.hasDragged && this.hoveredIndex >= 0 && this.onItemClick) {
      const media = this.medias[this.hoveredIndex]
      this.onItemClick({ image: media.imageSrc, text: media.text })
    } else {
      this.onCheck()
    }

    if (this.autoScrollTimer) clearTimeout(this.autoScrollTimer)
    this.autoScrollTimer = setTimeout(() => {
      this.isInteracting = false
      this.autoScrollSpeed = 0.15
    }, 3000)
  }

  private onWheel(e: WheelEvent) {
    e.preventDefault()
    const delta = e.deltaY || 0
    this.scroll.target += (delta > 0 ? this.scrollSpeed : -this.scrollSpeed) * 0.2
    this.isInteracting = true
    this.autoScrollSpeed = 0
    this.onCheckDebounce()
    if (this.autoScrollTimer) clearTimeout(this.autoScrollTimer)
    this.autoScrollTimer = setTimeout(() => {
      this.isInteracting = false
      this.autoScrollSpeed = 0.15
    }, 3000)
  }

  private onCheck() {
    if (!this.medias || !this.medias[0]) return
    const width = this.medias[0].width
    const itemIndex = Math.round(Math.abs(this.scroll.target) / width)
    const item = width * itemIndex
    this.scroll.target = this.scroll.target < 0 ? -item : item
  }

  private onResize() {
    this.screen = {
      width: this.container.clientWidth,
      height: this.container.clientHeight,
    }
    this.renderer.setSize(this.screen.width, this.screen.height)
    ;(this.camera as any).perspective({
      aspect: this.screen.width / this.screen.height,
    })
    const fov = ((this.camera as any).fov * Math.PI) / 180
    const height = 2 * Math.tan(fov / 2) * this.camera.position.z
    const width = height * (this.camera as any).aspect
    this.viewport = { width, height }
    if (this.medias) {
      this.medias.forEach(media => media.onResize({ screen: this.screen, viewport: this.viewport }))
    }
  }

  private update() {
    // Pause auto-scroll when hovering an image
    const shouldAutoScroll = !this.isInteracting && !this.reducedMotion && this.hoveredIndex < 0
    if (shouldAutoScroll) {
      this.scroll.target += this.autoScrollSpeed * 0.02
    }

    this.scroll.current = lerp(this.scroll.current, this.scroll.target, this.scroll.ease)
    const direction = this.scroll.current > this.scroll.last ? 'right' : 'left'

    if (this.medias) {
      this.medias.forEach(media => media.update(this.scroll, direction))
    }

    // Hit test after positions are updated
    this.hitTest()

    this.renderer.render({ scene: this.scene, camera: this.camera })
    this.scroll.last = this.scroll.current

    if (this.reducedMotion) return
    this.raf = window.requestAnimationFrame(this.boundUpdate)
  }

  private addEventListeners() {
    window.addEventListener('resize', this.boundOnResize)
    this.container.addEventListener('wheel', this.boundOnWheel, { passive: false })
    this.container.addEventListener('mousedown', this.boundOnTouchDown)
    window.addEventListener('mousemove', this.boundOnTouchMove)
    window.addEventListener('mouseup', this.boundOnTouchUp)
    this.container.addEventListener('touchstart', this.boundOnTouchDown, { passive: true })
    window.addEventListener('touchmove', this.boundOnTouchMove, { passive: true })
    window.addEventListener('touchend', this.boundOnTouchUp)
    this.container.addEventListener('mousemove', this.boundOnMouseMove)
    this.container.addEventListener('mouseleave', this.boundOnMouseLeave)
  }

  destroy() {
    if (this.autoScrollTimer) {
      clearTimeout(this.autoScrollTimer)
      this.autoScrollTimer = null
    }
    window.cancelAnimationFrame(this.raf)
    window.removeEventListener('resize', this.boundOnResize)
    this.container.removeEventListener('wheel', this.boundOnWheel)
    this.container.removeEventListener('mousedown', this.boundOnTouchDown)
    window.removeEventListener('mousemove', this.boundOnTouchMove)
    window.removeEventListener('mouseup', this.boundOnTouchUp)
    this.container.removeEventListener('touchstart', this.boundOnTouchDown)
    window.removeEventListener('touchmove', this.boundOnTouchMove)
    window.removeEventListener('touchend', this.boundOnTouchUp)
    this.container.removeEventListener('mousemove', this.boundOnMouseMove)
    this.container.removeEventListener('mouseleave', this.boundOnMouseLeave)
    // Clear media references to help GC reclaim textures/programs
    this.medias.length = 0
    // Remove all children from scene
    if (this.scene) {
      while (this.scene.children && this.scene.children.length) {
        this.scene.removeChild(this.scene.children[0])
      }
    }
    if (this.gl?.canvas?.parentNode) {
      this.gl.canvas.parentNode.removeChild(this.gl.canvas)
    }
    this.gl?.getExtension('WEBGL_lose_context')?.loseContext()
  }
}

// ─── Vue integration ───

let app: GalleryApp | null = null
let isUnmounted = false

function handleItemClick(item: { image: string; text: string }) {
  if (!isValidImageSrc(item.image)) return
  lightboxImage.value = item.image
  lightboxTitle.value = item.text || null
}

function createApp() {
  if (isUnmounted || !containerRef.value || filteredItems.value.length === 0) return
  const items = filteredItems.value.map(item => ({
    image: item.image,
    text: item.title,
  }))
  app = new GalleryApp(
    containerRef.value,
    items,
    3,
    textColor.value,
    0.05,
    'bold 24px sans-serif',
    2,
    0.05,
    prefersReducedMotion,
    handleItemClick,
  )
}

function destroyApp() {
  if (app) {
    app.destroy()
    app = null
  }
}

function updateGalleryHeight() {
  if (typeof window === 'undefined') return
  const w = window.innerWidth
  if (w < 640) galleryHeight.value = 280
  else if (w < 1024) galleryHeight.value = 350
  else galleryHeight.value = 400
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') lightboxImage.value = null
}

watch(filteredItems, async () => {
  destroyApp()
  await nextTick()
  createApp()
})

onMounted(async () => {
  updateGalleryHeight()
  window.addEventListener('resize', updateGalleryHeight)
  document.addEventListener('keydown', onKeydown)
  await nextTick()
  createApp()
})

onUnmounted(() => {
  isUnmounted = true
  destroyApp()
  window.removeEventListener('resize', updateGalleryHeight)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.gallery-container {
  cursor: grab;
  user-select: none;
  -webkit-user-select: none;
  touch-action: none;
}

.gallery-container:active {
  cursor: grabbing;
}

.lightbox-enter-active {
  transition: opacity 0.2s ease;
}

.lightbox-leave-active {
  transition: opacity 0.15s ease;
}

.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}
</style>
