import { onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'

// One listener for the whole window: every guard asks the same question.
const pointer = { x: -1, y: -1 }
let tracked = 0

function track(e: PointerEvent) {
  pointer.x = e.clientX
  pointer.y = e.clientY
}

function pointerInside(node: HTMLElement): boolean {
  const r = node.getBoundingClientRect()
  return pointer.x >= r.left && pointer.x <= r.right && pointer.y >= r.top && pointer.y <= r.bottom
}

// Whether the pointer arrived at `el` by moving rather than `el` appearing under it (a popover
// opening where the user clicked). Starts unarmed then, and arms on pointerleave.
export function useHoverArrival(el: Ref<HTMLElement | null>) {
  const armed = ref(false)

  onMounted(() => {
    if (++tracked === 1) window.addEventListener('pointermove', track, true)
  })

  onBeforeUnmount(() => {
    if (--tracked === 0) window.removeEventListener('pointermove', track, true)
  })

  // Post-flush: a box means nothing before layout.
  watch(
    el,
    (node) => {
      armed.value = node !== null && !pointerInside(node)
    },
    { flush: 'post' }
  )

  return armed
}
