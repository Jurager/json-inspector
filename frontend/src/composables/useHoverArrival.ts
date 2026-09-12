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

/**
 * Whether the pointer arrived at `el` by moving, rather than `el` appearing
 * under a pointer that was already sitting there — a popover opening where the
 * user just clicked. Until it has, a hint for that element fires on the first
 * twitch of the mouse, which reads as a glitch rather than as a hint.
 *
 * A control that appears with the pointer already inside starts unarmed, and
 * arms on the first pointerleave: the pointer has to come back in, which is the
 * only thing that tells "they hovered it" from "it showed up underneath them".
 */
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
