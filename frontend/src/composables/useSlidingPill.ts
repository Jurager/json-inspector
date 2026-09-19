import { onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'

// The pill that slides under the chosen option of a segmented control.
//
// It is measured rather than computed: the options are as wide as their own words — «Параметры 3» is
// not the width of «Auth» — and the element itself is the only thing that knows how wide it came out.
// One pill moves between the choices instead of each choice painting itself, because the eye follows
// the movement and lands where the choice went.
//
// The pill is placed from the chosen option's offset inside `root`, so `root` has to be the control
// and has to be positioned: that is what makes the offsets the pill's own coordinate space.
export function useSlidingPill(root: Ref<HTMLElement | null>, chosen: string, follow: () => unknown) {
  const style = ref<Record<string, string>>({})
  // The first placement is made without the transition: a pill animating in from the corner would be
  // a second thing happening on a control that has only just been drawn.
  const ready = ref(false)

  // A template ref is the element when it is on one and the component's instance when it is on a
  // primitive — the popover's anchor is reka's, and the control there is the element it renders. The
  // two are read the same way so that a caller does not have to know which kind of thing it holds.
  function element(): HTMLElement | null {
    const held = root.value as unknown
    if (held instanceof HTMLElement) return held
    const inner = (held as { $el?: unknown } | null)?.$el
    return inner instanceof HTMLElement ? inner : null
  }

  function place() {
    const option = element()?.querySelector<HTMLElement>(chosen)
    // A control can have nothing chosen — the request bar's sections are raised only while one of
    // them is open — and the pill then fades where it stands rather than jumping to a corner.
    if (!option) {
      style.value = { ...style.value, opacity: '0' }
      if (!ready.value) ready.value = true
      return
    }
    style.value = {
      transform: `translate(${option.offsetLeft}px, ${option.offsetTop}px)`,
      width: `${option.offsetWidth}px`,
      height: `${option.offsetHeight}px`,
      opacity: '1',
    }
    if (!ready.value) ready.value = true
  }

  // `follow` is what the placement depends on beyond the choice itself: an option whose words changed
  // — a counter appearing beside a label — is a different width in the same place. It runs after the
  // DOM has been patched, because offsets are what is being read.
  watch([follow, root], place, { flush: 'post', immediate: true })

  let resizes: ResizeObserver | null = null
  let choices: MutationObserver | null = null

  onMounted(() => {
    const box = element()
    if (!box) return

    // A control that is resized without anything being clicked — the window narrowed, the panel
    // dragged — leaves the pill where it was, which is a pill under the wrong option.
    if (typeof ResizeObserver !== 'undefined') {
      resizes = new ResizeObserver(place)
      resizes.observe(box)
    }

    // And the choice itself: a reka control says which option is on with an attribute rather than
    // through anything this composable could watch, so the control is watched instead. The pill's own
    // style is written by the placement, so a mutation that is only that would be a loop.
    if (typeof MutationObserver !== 'undefined') {
      choices = new MutationObserver((records) => {
        const pill = box.querySelector('.slide-mark')
        if (records.every((r) => r.target === pill)) return
        place()
      })
      choices.observe(box, { attributes: true, subtree: true, attributeFilter: ['class', 'data-state'] })
    }

    place()
  })

  onBeforeUnmount(() => {
    resizes?.disconnect()
    choices?.disconnect()
  })

  return { style, ready }
}
