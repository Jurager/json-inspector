<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { onPaint, useTheme, type Theme } from '../../composables/useTheme'
import { fillArrival } from '../../lib/themeWipe'

const { theme, setTheme, isSwitching } = useTheme()

// The handoff's order: light, dark, system.
const OPTIONS: { value: Theme; icon: string; title: string }[] = [
  { value: 'light', icon: 'sun', title: 'Светлая' },
  { value: 'dark', icon: 'moon', title: 'Тёмная' },
  { value: 'system', icon: 'monitor', title: 'Системная' },
]

const rootEl = ref<HTMLElement | null>(null)

const activeIndex = computed(() => OPTIONS.findIndex((o) => o.value === theme.value))

// What the pill shows. It follows the theme one repaint later, inside the view transition's own
// callback (see `onPaint`), so that is where the pill is for each of the two snapshots: still in
// the old segment for the outgoing one, in the new one for the incoming. The buttons and the
// glyph copies above the pill read it too — in the frozen window the three of them have to agree.
const shownIndex = ref(activeIndex.value)

const pill = () => rootEl.value?.querySelector<HTMLElement>('.theme-indicator') ?? null
const options = () => Array.from(rootEl.value?.querySelectorAll<HTMLElement>('.theme-option') ?? [])

// Runs inside the transition's callback, where the pill is handed over as a layer of its own — for
// the incoming snapshot only: naming it in both would make the layer lay itself out on the segment
// it lands on in the outgoing snapshot too (Chrome does that, the spec does not) and the pill would
// come back to where it started. The layer's animation is measured here as well, while the pill is
// still where the user left it: how far it has to run, and when it may lift off — the moment the
// fill's front reaches it, since leaving earlier would uncover the outgoing snapshot's own pill at
// the old segment for the rest of the fill.
function arm(wiping: boolean) {
  const from = pill()?.getBoundingClientRect()
  const to = options()[activeIndex.value]?.getBoundingClientRect()
  if (wiping && from && to) {
    const root = document.documentElement.style
    root.setProperty('--pill-shift', `${Math.round(from.left - to.left)}px`)
    const arrival = fillArrival(from.right, from.bottom, window.innerWidth, window.innerHeight)
    root.setProperty('--wipe-arrival', `${arrival}ms`)
    pill()?.style.setProperty('view-transition-name', 'theme-indicator')
  }
  shownIndex.value = activeIndex.value
}

onBeforeUnmount(onPaint(arm))

const indicatorStyle = computed(() => ({
  transform: `translateX(calc(${shownIndex.value} * (100% + 2px)))`,
}))

// The pill reaches its segment inside the same frame; what the user sees travelling is the layer
// above, animated in style.css from the offset `arm` measured.
function pick(value: Theme) {
  if (value !== theme.value) setTheme(value)
}

// Nothing in the window answers a pointer while a wipe is on screen: Chromium hands the whole
// hit test to the transition, so events arrive on `<html>` and `elementFromPoint` answers
// `<html>` too. The switch is the one control the user needs over the wipe — it started it —
// so while one runs it answers by coordinates, measured before the window froze.
let segmentRects: DOMRect[] = []

function onPointerDown(e: PointerEvent) {
  const at = segmentRects.findIndex(
    (r) => e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom
  )
  if (at !== -1) pick(OPTIONS[at].value)
}

watch(isSwitching, (switching) => {
  if (switching) {
    segmentRects = options().map((o) => o.getBoundingClientRect())
    document.addEventListener('pointerdown', onPointerDown, true)
    return
  }
  document.removeEventListener('pointerdown', onPointerDown, true)
  // The layer's name is only for the wipe that just ended: the window is live again and the pill
  // is an ordinary element until the next one.
  pill()?.style.removeProperty('view-transition-name')
})

onBeforeUnmount(() => document.removeEventListener('pointerdown', onPointerDown, true))
</script>

<template>
  <div ref="rootEl" class="theme-switch" title="Тема оформления">
    <span class="theme-indicator" :style="indicatorStyle"></span>
    <!-- The pill is a layer of its own while the wipe runs, and it is opaque: it paints over the
         segment it lands on, glyph included. These copies ride on a layer above it instead, so the
         background is what travels and the glyphs stay put. Only while a wipe runs — the rest of
         the time the buttons draw them. -->
    <span v-if="isSwitching" class="theme-glyphs" aria-hidden="true">
      <span
        v-for="(o, i) in OPTIONS"
        :key="o.value"
        class="theme-glyph"
        :class="{ active: i === shownIndex }"
      >
        <Icon :name="o.icon" :size="13" :stroke-width="1.8" />
      </span>
    </span>
    <button
      v-for="(o, i) in OPTIONS"
      :key="o.value"
      class="theme-option"
      :class="{ active: i === shownIndex }"
      :title="o.title"
      @click="pick(o.value)"
    >
      <Icon :name="o.icon" :size="13" :stroke-width="1.8" />
    </button>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.theme-switch {
  @apply relative flex flex-none items-center gap-0.5 h-[26px] p-0.5 rounded-[7px] bg-bg-active;
}

/* One element that holds the active segment, as in the auth panel. No CSS transition: the pill
   has to be in place before the window is snapshotted, or the wipe would catch it half way —
   the movement the user sees is this element's own layer in the transition, animated in
   style.css. */
.theme-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 inline-flex items-center justify-center bg-bg-panel rounded-[5px];
  width: 24px;
  box-shadow: var(--shadow-btn);
}

/* Laid out exactly as the row of buttons (same padding, same gap, same box), so the copies land
   on the glyphs they stand for whatever the icon is. */
.theme-glyphs {
  @apply absolute inset-0.5 flex items-center gap-0.5;
  view-transition-name: theme-glyphs;
}

.theme-glyph {
  @apply w-6 h-[22px] flex-none inline-flex items-center justify-center text-text-tertiary;
}

.theme-option {
  @apply relative w-6 h-[22px] flex-none inline-flex items-center justify-center border-none bg-transparent rounded-[5px] text-text-tertiary cursor-pointer;
  transition: color 0.18s ease, transform 0.1s ease;
  --wails-draggable: no-drag;
}

.theme-option:hover,
.theme-option.active,
.theme-glyph.active {
  @apply text-text;
}

/* Real-time feedback, not tied to the wipe: the click has to read as answered right away even
   though the pill itself waits for the fill to reach it (see style.css, pill-slide). */
.theme-option:active {
  transform: scale(0.86);
}
</style>
