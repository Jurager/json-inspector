import { onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'

// Where the pointer was last seen. One listener for the whole window rather
// than one per guarded control: they all ask the same question.
const pointer = { x: -1, y: -1 }
let tracked = 0

function track(e: PointerEvent) {
  pointer.x = e.clientX
  pointer.y = e.clientY
}

/**
 * Whether the pointer arrived at `el` by moving there, as opposed to `el`
 * appearing under a pointer that was already sitting in that spot — a popover
 * opening right where the user clicked its chip, a row's ✕ landing under the
 * cursor. In the second case the first twitch of the mouse fires a hint nobody
 * asked for, and it reads as a glitch rather than as a hint.
 *
 * So a control starts unarmed whenever the pointer is already inside its box,
 * and arms on the first pointerleave: after that the pointer has to come back
 * in, which is the only thing that tells "they hovered it" from "it showed up
 * underneath them".
 */
export function useHoverArrival(el: Ref<HTMLElement | null>) {
  const armed = ref(false)

  onMounted(() => {
    if (++tracked === 1) window.addEventListener('pointermove', track, true)
  })

  onBeforeUnmount(() => {
    if (--tracked === 0) window.removeEventListener('pointermove', track, true)
  })

  // Post-flush: the element has to be laid out before its box means anything.
  watch(
    el,
    (node) => {
      if (!node) {
        armed.value = false
        return
      }
      const r = node.getBoundingClientRect()
      const inside =
        pointer.x >= r.left && pointer.x <= r.right && pointer.y >= r.top && pointer.y <= r.bottom
      armed.value = !inside
    },
    { flush: 'post' }
  )

  return armed
}
