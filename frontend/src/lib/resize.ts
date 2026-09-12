import type { Ref } from 'vue'

// `direction` is which edge the handle sits on: the side list's right edge (+1),
// the inspector's left (-1). Sharing one sign would resize one of them backwards.
export function makeSideResizer(
  width: Ref<number>,
  min: number,
  max: number,
  direction: 1 | -1 = 1
) {
  let state: { startX: number; startWidth: number } | null = null
  return {
    start(e: MouseEvent) {
      state = { startX: e.clientX, startWidth: width.value }
      document.body.style.cursor = 'col-resize'
      document.body.style.userSelect = 'none'
    },
    move(e: MouseEvent) {
      if (!state) return
      const delta = (e.clientX - state.startX) * direction
      width.value = Math.min(max, Math.max(min, state.startWidth + delta))
    },
    stop() {
      state = null
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    },
  }
}
