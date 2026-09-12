import { onBeforeUnmount, onMounted, type Ref } from 'vue'

export function useResizableWidth(
  width: Ref<number>,
  { min, max, side }: { min: number; max: number; side: 'left' | 'right' }
) {
  let drag: { startX: number; startWidth: number } | null = null

  function onMove(e: MouseEvent) {
    if (!drag) return
    const delta = (e.clientX - drag.startX) * (side === 'left' ? 1 : -1)
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
