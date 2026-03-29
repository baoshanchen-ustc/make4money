<template>
  <canvas ref="canvasRef" :width="width" :height="height" />
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'

const props = withDefaults(defineProps<{
  data: number[]      // 7 个数值（最旧→最新）
  width?: number
  height?: number
  color?: string
}>(), {
  width: 80,
  height: 28,
  color: '#3b82f6',
})

const canvasRef = ref<HTMLCanvasElement | null>(null)

function draw() {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  const { data, width, height, color } = props
  ctx.clearRect(0, 0, width, height)

  if (data.length < 2) return

  const max = Math.max(...data, 1)
  const min = Math.min(...data, 0)
  const range = max - min || 1
  const pad = 3

  const points = data.map((v, i) => ({
    x: pad + (i / (data.length - 1)) * (width - pad * 2),
    y: height - pad - ((v - min) / range) * (height - pad * 2),
  }))

  // Fill area
  ctx.beginPath()
  ctx.moveTo(points[0].x, height - pad)
  points.forEach(p => ctx.lineTo(p.x, p.y))
  ctx.lineTo(points[points.length - 1].x, height - pad)
  ctx.closePath()
  ctx.fillStyle = color + '22'
  ctx.fill()

  // Line
  ctx.beginPath()
  ctx.moveTo(points[0].x, points[0].y)
  points.slice(1).forEach(p => ctx.lineTo(p.x, p.y))
  ctx.strokeStyle = color
  ctx.lineWidth = 1.5
  ctx.lineJoin = 'round'
  ctx.stroke()
}

onMounted(draw)
watch(() => props.data, draw, { deep: true })
</script>
