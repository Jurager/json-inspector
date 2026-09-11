import type { Ref } from 'vue'

// A reusable column-resize controller. Both the side list and the inspector
// need the same drag behaviour, so it lives here instead of being copied.
export function makeSideResizer(width: Ref<number>, min: number, max: number) {
  let state: { startX: number; startWidth: number } | null = null
  return {
    start(e: MouseEvent) {
      state = { startX: e.clientX, startWidth: width.value }
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
    },
    move(e: MouseEvent) {
      if (!state) return
      const delta = e.clientX - state.startX
      width.value = Math.min(max, Math.max(min, state.startWidth + delta))
    },
    stop() {
      state = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    },
  }
}
