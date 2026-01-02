import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import './style.css'
import { useAnalytics } from './composables/useAnalytics'

const app = createApp(App)
const head = createHead()

app.use(createPinia())
app.use(router)
app.use(i18n)
app.use(head)

// Initialize analytics if IDs are provided
useAnalytics({
  googleAnalyticsId: import.meta.env.VITE_GOOGLE_ANALYTICS_ID,
  baiduAnalyticsId: import.meta.env.VITE_BAIDU_ANALYTICS_ID
})

// 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
router.isReady().then(() => {
  app.mount('#app')

  // 触发预渲染完成事件
  if (typeof window !== 'undefined') {
    document.dispatchEvent(new Event('render-event'))
  }
})
