<template>
  <!--
    CCAI 品牌 Logo（行内 SVG，可缩放无依赖）
    设计：抽象神经网络节点图，5 个圆形节点 + 连线，蓝紫渐变色
    灵感源于 AI 模型网络 / 智能中转节点
    使用：<Logo :size="48" /> 或 <Logo :size="36" :rounded="false" />
  -->
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 48 48"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    role="img"
    aria-label="CCAI"
    class="shrink-0"
  >
    <defs>
      <radialGradient :id="`ccaiGrad${uid}`" cx="30%" cy="20%" r="80%">
        <stop offset="0%" :stop-color="accentColor" stop-opacity="0.95" />
        <stop offset="55%" :stop-color="primaryColor" stop-opacity="0.9" />
        <stop offset="100%" :stop-color="deepColor" stop-opacity="0.85" />
      </radialGradient>
    </defs>

    <rect width="48" height="48" :rx="rounded ? 12 : 0" :fill="bgColor" />

    <!-- Edges（连线）：位于节点后层，淡透明 -->
    <g :stroke="primaryColor" stroke-width="1.2" stroke-linecap="round" opacity="0.55">
      <line x1="14" y1="16" x2="24" y2="24" />
      <line x1="34" y1="14" x2="24" y2="24" />
      <line x1="12" y1="30" x2="24" y2="24" />
      <line x1="35" y1="32" x2="24" y2="24" />
      <line x1="22" y1="36" x2="24" y2="24" />
      <line x1="14" y1="16" x2="34" y2="14" />
      <line x1="12" y1="30" x2="22" y2="36" />
    </g>

    <!-- 次级节点（小圆） -->
    <circle cx="14" cy="16" r="3" :fill="primaryColor" opacity="0.85" />
    <circle cx="34" cy="14" r="4" :fill="accentColor" opacity="0.9" />
    <circle cx="12" cy="30" r="3.5" :fill="purpleColor" opacity="0.85" />
    <circle cx="35" cy="32" r="3" :fill="primaryColor" opacity="0.85" />
    <circle cx="22" cy="36" r="2.8" :fill="purpleColor" opacity="0.9" />

    <!-- 中心主节点（最大，径向渐变） -->
    <circle cx="24" cy="24" r="5.5" :fill="`url(#ccaiGrad${uid})`" />
  </svg>
</template>

<script setup lang="ts">
// uid 保证同页多实例的 radialGradient id 不冲突
const uid = Math.random().toString(36).slice(2, 8)

withDefaults(
  defineProps<{
    size?: number
    rounded?: boolean
    /** 深色底 */
    bgColor?: string
    /** 主蓝（blue-500） */
    primaryColor?: string
    /** 浅蓝（sky-400）—— 节点高光 */
    accentColor?: string
    /** 深蓝（blue-700） */
    deepColor?: string
    /** 紫色辅助（violet-500） */
    purpleColor?: string
  }>(),
  {
    size: 48,
    rounded: true,
    bgColor: '#0f172a',
    primaryColor: '#3b82f6',
    accentColor: '#60a5fa',
    deepColor: '#1d4ed8',
    purpleColor: '#8b5cf6'
  }
)
</script>
