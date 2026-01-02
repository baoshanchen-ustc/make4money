import { useHead } from '@unhead/vue'

export interface AnalyticsConfig {
  googleAnalyticsId?: string
  baiduAnalyticsId?: string
}

// Initialize analytics
export function useAnalytics(config: AnalyticsConfig) {
  const scripts: any[] = []

  // Google Analytics (GA4)
  if (config.googleAnalyticsId) {
    scripts.push(
      {
        src: `https://www.googletagmanager.com/gtag/js?id=${config.googleAnalyticsId}`,
        async: true
      },
      {
        children: `
          window.dataLayer = window.dataLayer || [];
          function gtag(){dataLayer.push(arguments);}
          gtag('js', new Date());
          gtag('config', '${config.googleAnalyticsId}');
        `
      }
    )
  }

  // Baidu Analytics (百度统计)
  if (config.baiduAnalyticsId) {
    scripts.push({
      children: `
        var _hmt = _hmt || [];
        (function() {
          var hm = document.createElement("script");
          hm.src = "https://hm.baidu.com/hm.js?${config.baiduAnalyticsId}";
          var s = document.getElementsByTagName("script")[0];
          s.parentNode.insertBefore(hm, s);
        })();
      `
    })
  }

  if (scripts.length > 0) {
    useHead({
      script: scripts
    })
  }
}

// Track custom events (works with both GA and Baidu)
export function trackEvent(
  category: string,
  action: string,
  label?: string,
  value?: number
) {
  // Google Analytics
  if (typeof window !== 'undefined' && (window as any).gtag) {
    ;(window as any).gtag('event', action, {
      event_category: category,
      event_label: label,
      value: value
    })
  }

  // Baidu Analytics
  if (typeof window !== 'undefined' && (window as any)._hmt) {
    ;(window as any)._hmt.push(['_trackEvent', category, action, label, value])
  }
}

// Track page views (for SPA navigation)
export function trackPageView(url: string, title?: string) {
  // Google Analytics
  if (typeof window !== 'undefined' && (window as any).gtag) {
    ;(window as any).gtag('config', (window as any).GA_MEASUREMENT_ID, {
      page_path: url,
      page_title: title
    })
  }

  // Baidu Analytics
  if (typeof window !== 'undefined' && (window as any)._hmt) {
    ;(window as any)._hmt.push(['_trackPageview', url])
  }
}
