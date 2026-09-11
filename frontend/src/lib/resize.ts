import type { Ref } from 'vue'

// A reusable column-resize controller. Both the side list and the inspector
// need the same drag behaviour, so it lives here instead of being copied.
//
// `direction` accounts for which edge the handle sits on: the side list's
// handle is its right edge (drag right = wider, +1), while the inspector's
// handle is its left edge (drag right = narrower, -1). Without it the two
// would share the same arithmetic and one of them would resize backwards.
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
