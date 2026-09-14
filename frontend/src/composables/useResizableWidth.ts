import { onBeforeUnmount, onMounted, type Ref } from 'vue'

type Side = 'left' | 'right'

export function useResizableWidth(
  width: Ref<number>,
  { min, max, side }: { min: number; max: number; side: Side | (() => Side) }
) {
  let drag: { startX: number; startWidth: number } | null = null

  function onMove(e: MouseEvent) {
    if (!drag) return
    // The edge is read per drag, not per call: the same panel is dragged from either side of the
    // work area, and the direction a pointer has to travel to grow it flips with it.
    const edge = typeof side === 'function' ? side() : side
    const delta = (e.clientX - drag.startX) * (edge === 'left' ? 1 : -1)
    width.value = Math.min(max, Math.max(min, drag.startWidth + delta))
  }

  function endDrag() {
    drag = null
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }

  function startDrag(e: MouseEvent) {
    drag = { startX: e.clientX, startWidth: width.value }
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  onMounted(() => {
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', endDrag)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', endDrag)
    endDrag()
  })

  return { startDrag }
}
