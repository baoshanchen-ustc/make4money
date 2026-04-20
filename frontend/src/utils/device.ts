/**
 * Detect whether the current device is mobile.
 * Uses userAgentData first, then falls back to UA / touch heuristics for
 * iPadOS and embedded browsers that often omit a clear "Mobile" token.
 */
export function isMobileDevice(): boolean {
  if (typeof navigator === 'undefined' || typeof window === 'undefined') {
    return false
  }

  const nav = navigator as unknown as Record<string, unknown>
  if (nav.userAgentData && typeof (nav.userAgentData as Record<string, unknown>).mobile === 'boolean') {
    return (nav.userAgentData as Record<string, unknown>).mobile as boolean
  }

  const userAgent = navigator.userAgent || ''
  if (/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini|Mobile|HarmonyOS|AlipayClient|MicroMessenger/i.test(userAgent)) {
    return true
  }

  const platform = navigator.platform || ''
  if (/MacIntel/i.test(platform) && (navigator.maxTouchPoints || 0) > 1) {
    return true
  }

  const coarsePointer = typeof window.matchMedia === 'function'
    ? window.matchMedia('(pointer: coarse)').matches
    : false
  if (coarsePointer && Math.min(window.innerWidth, window.innerHeight) <= 1024) {
    return true
  }

  return (navigator.maxTouchPoints || 0) > 1 && Math.min(window.innerWidth, window.innerHeight) <= 834
}
