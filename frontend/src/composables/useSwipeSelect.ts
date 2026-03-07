import { ref, onMounted, onUnmounted, type Ref } from 'vue'

/**
 * WeChat-style swipe/drag to select rows in a DataTable,
 * with a semi-transparent marquee overlay showing the selection area.
 *
 * Features:
 *  - Start dragging from anywhere on the page (not just inside the table)
 *  - Mouse wheel scrolling continues selecting new rows
 *  - Auto-scroll when dragging near viewport edges
 *  - 5px drag threshold to avoid accidental selection on click
 *
 * Usage:
 *   const containerRef = ref<HTMLElement | null>(null)
 *   useSwipeSelect(containerRef, {
 *     isSelected: (id) => selIds.value.includes(id),
 *     select: (id) => { if (!selIds.value.includes(id)) selIds.value.push(id) },
 *     deselect: (id) => { selIds.value = selIds.value.filter(x => x !== id) },
 *   })
 *
 * Wrap <DataTable> with <div ref="containerRef">...</div>
 * DataTable rows must have data-row-id attribute.
 */
export interface SwipeSelectAdapter {
  isSelected: (id: number) => boolean
  select: (id: number) => void
  deselect: (id: number) => void
}

export function useSwipeSelect(
  containerRef: Ref<HTMLElement | null>,
  adapter: SwipeSelectAdapter
) {
  const isDragging = ref(false)

  let dragMode: 'select' | 'deselect' = 'select'
  let startRowIndex = -1
  let lastEndIndex = -1
  let startY = 0
  let lastMouseY = 0
  let pendingStartY = 0
  let initialSelectedSnapshot = new Map<number, boolean>()
  let cachedRows: HTMLElement[] = []
  let marqueeEl: HTMLDivElement | null = null

  const DRAG_THRESHOLD = 5
  const SCROLL_ZONE = 60
  const SCROLL_SPEED = 8

  function getDataRows(): HTMLElement[] {
    const container = containerRef.value
    if (!container) return []
    return Array.from(container.querySelectorAll('tbody tr[data-row-id]'))
  }

  function getRowId(el: HTMLElement): number | null {
    const raw = el.getAttribute('data-row-id')
    if (raw === null) return null
    const id = Number(raw)
    return Number.isFinite(id) ? id : null
  }

  /** Find the row index closest to a viewport Y coordinate. */
  function findRowIndexAtY(clientY: number): number {
    if (cachedRows.length === 0) return -1
    // Direct hit
    for (let i = 0; i < cachedRows.length; i++) {
      const rect = cachedRows[i].getBoundingClientRect()
      if (clientY >= rect.top && clientY <= rect.bottom) return i
    }
    // Above all rows
    if (clientY < cachedRows[0].getBoundingClientRect().top) return 0
    // Below all rows
    if (clientY > cachedRows[cachedRows.length - 1].getBoundingClientRect().bottom) {
      return cachedRows.length - 1
    }
    // In a gap — find nearest
    let bestIdx = 0
    let bestDist = Infinity
    for (let i = 0; i < cachedRows.length; i++) {
      const rect = cachedRows[i].getBoundingClientRect()
      const dist = Math.abs(clientY - (rect.top + rect.bottom) / 2)
      if (dist < bestDist) { bestDist = dist; bestIdx = i }
    }
    return bestIdx
  }

  // --- Prevent text selection via selectstart (no body style mutation) ---
  function onSelectStart(e: Event) { e.preventDefault() }

  // --- Marquee overlay ---
  function createMarquee() {
    marqueeEl = document.createElement('div')
    const isDark = document.documentElement.classList.contains('dark')
    Object.assign(marqueeEl.style, {
      position: 'fixed',
      background: isDark ? 'rgba(96, 165, 250, 0.15)' : 'rgba(59, 130, 246, 0.12)',
      border: isDark ? '1.5px solid rgba(96, 165, 250, 0.5)' : '1.5px solid rgba(59, 130, 246, 0.4)',
      borderRadius: '4px',
      pointerEvents: 'none',
      zIndex: '9999',
      transition: 'none',
    })
    document.body.appendChild(marqueeEl)
  }

  function updateMarquee(currentY: number) {
    if (!marqueeEl || !containerRef.value) return
    const containerRect = containerRef.value.getBoundingClientRect()
    const top = Math.min(startY, currentY)
    const bottom = Math.max(startY, currentY)
    marqueeEl.style.left = containerRect.left + 'px'
    marqueeEl.style.width = containerRect.width + 'px'
    marqueeEl.style.top = top + 'px'
    marqueeEl.style.height = (bottom - top) + 'px'
  }

  function removeMarquee() {
    if (marqueeEl) { marqueeEl.remove(); marqueeEl = null }
  }

  // --- Row selection logic ---
  function applyRange(endIndex: number) {
    if (startRowIndex < 0 || endIndex < 0) return
    const rangeMin = Math.min(startRowIndex, endIndex)
    const rangeMax = Math.max(startRowIndex, endIndex)
    const prevMin = lastEndIndex >= 0 ? Math.min(startRowIndex, lastEndIndex) : rangeMin
    const prevMax = lastEndIndex >= 0 ? Math.max(startRowIndex, lastEndIndex) : rangeMax
    const lo = Math.min(rangeMin, prevMin)
    const hi = Math.max(rangeMax, prevMax)

    for (let i = lo; i <= hi && i < cachedRows.length; i++) {
      const id = getRowId(cachedRows[i])
      if (id === null) continue
      if (i >= rangeMin && i <= rangeMax) {
        if (dragMode === 'select') adapter.select(id)
        else adapter.deselect(id)
      } else {
        const wasSelected = initialSelectedSnapshot.get(id) ?? false
        if (wasSelected) adapter.select(id)
        else adapter.deselect(id)
      }
    }
    lastEndIndex = endIndex
  }

  // --- Scrollable parent ---
  function getScrollParent(el: HTMLElement): HTMLElement {
    let parent = el.parentElement
    while (parent && parent !== document.documentElement) {
      const { overflow, overflowY } = getComputedStyle(parent)
      if (/(auto|scroll)/.test(overflow + overflowY)) return parent
      parent = parent.parentElement
    }
    return document.documentElement
  }

  // =============================================
  // Phase 1: detect drag threshold (5px movement)
  // =============================================
  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return
    if (!containerRef.value) return

    // Skip clicks on scrollbar area (clientWidth excludes scrollbar)
    const docEl = document.documentElement
    if (e.clientX >= docEl.clientWidth || e.clientY >= docEl.clientHeight) return

    const target = e.target as HTMLElement
    if (target.closest('button, a, input, select, textarea, [role="button"], [role="menuitem"], [role="combobox"], [role="dialog"]')) return

    cachedRows = getDataRows()
    if (cachedRows.length === 0) return

    pendingStartY = e.clientY
    document.addEventListener('mousemove', onThresholdMove)
    document.addEventListener('mouseup', onThresholdUp)
  }

  function onThresholdMove(e: MouseEvent) {
    if (Math.abs(e.clientY - pendingStartY) < DRAG_THRESHOLD) return
    // Threshold exceeded — begin actual drag
    document.removeEventListener('mousemove', onThresholdMove)
    document.removeEventListener('mouseup', onThresholdUp)

    beginDrag(pendingStartY)

    // Process the move that crossed the threshold
    lastMouseY = e.clientY
    updateMarquee(e.clientY)
    const rowIdx = findRowIndexAtY(e.clientY)
    if (rowIdx >= 0) applyRange(rowIdx)
    autoScroll(e)

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
    document.addEventListener('wheel', onWheel, { passive: true })
  }

  function onThresholdUp() {
    document.removeEventListener('mousemove', onThresholdMove)
    document.removeEventListener('mouseup', onThresholdUp)
    cachedRows = []
  }

  // ============================
  // Phase 2: actual drag session
  // ============================
  function beginDrag(clientY: number) {
    startRowIndex = findRowIndexAtY(clientY)
    const startRowId = startRowIndex >= 0 ? getRowId(cachedRows[startRowIndex]) : null
    dragMode = (startRowId !== null && adapter.isSelected(startRowId)) ? 'deselect' : 'select'

    initialSelectedSnapshot = new Map()
    for (const row of cachedRows) {
      const id = getRowId(row)
      if (id !== null) initialSelectedSnapshot.set(id, adapter.isSelected(id))
    }

    isDragging.value = true
    startY = clientY
    lastMouseY = clientY
    lastEndIndex = -1

    createMarquee()
    updateMarquee(clientY)
    applyRange(startRowIndex)
    // Prevent text selection during drag (no body style mutation)
    document.addEventListener('selectstart', onSelectStart)
  }

  function onMouseMove(e: MouseEvent) {
    if (!isDragging.value) return
    lastMouseY = e.clientY
    updateMarquee(e.clientY)
    const rowIdx = findRowIndexAtY(e.clientY)
    if (rowIdx >= 0) applyRange(rowIdx)
    autoScroll(e)
  }

  function onWheel() {
    if (!isDragging.value) return
    // After wheel scroll, rows shift in viewport — re-check selection
    requestAnimationFrame(() => {
      const rowIdx = findRowIndexAtY(lastMouseY)
      if (rowIdx >= 0) applyRange(rowIdx)
    })
  }

  function cleanupDrag() {
    isDragging.value = false
    startRowIndex = -1
    lastEndIndex = -1
    cachedRows = []
    initialSelectedSnapshot.clear()
    stopAutoScroll()
    removeMarquee()
    document.removeEventListener('selectstart', onSelectStart)
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    document.removeEventListener('wheel', onWheel)
  }

  function onMouseUp() {
    cleanupDrag()
  }

  // --- Auto-scroll logic ---
  let scrollRAF = 0

  function autoScroll(e: MouseEvent) {
    cancelAnimationFrame(scrollRAF)
    const container = containerRef.value
    if (!container) return
    const scrollEl = getScrollParent(container)

    let dy = 0
    if (scrollEl === document.documentElement) {
      if (e.clientY < SCROLL_ZONE) dy = -SCROLL_SPEED
      else if (e.clientY > window.innerHeight - SCROLL_ZONE) dy = SCROLL_SPEED
    } else {
      const rect = scrollEl.getBoundingClientRect()
      if (e.clientY < rect.top + SCROLL_ZONE) dy = -SCROLL_SPEED
      else if (e.clientY > rect.bottom - SCROLL_ZONE) dy = SCROLL_SPEED
    }

    if (dy !== 0) {
      const step = () => {
        scrollEl.scrollTop += dy
        // After each scroll frame, re-check which rows are in range
        const rowIdx = findRowIndexAtY(lastMouseY)
        if (rowIdx >= 0) applyRange(rowIdx)
        scrollRAF = requestAnimationFrame(step)
      }
      scrollRAF = requestAnimationFrame(step)
    }
  }

  function stopAutoScroll() {
    cancelAnimationFrame(scrollRAF)
  }

  // --- Lifecycle ---
  onMounted(() => {
    // Listen on document so drag can start from anywhere on the page
    document.addEventListener('mousedown', onMouseDown)
  })

  onUnmounted(() => {
    document.removeEventListener('mousedown', onMouseDown)
    // Clean up any in-progress drag state
    document.removeEventListener('mousemove', onThresholdMove)
    document.removeEventListener('mouseup', onThresholdUp)
    cleanupDrag()
  })

  return { isDragging }
}
