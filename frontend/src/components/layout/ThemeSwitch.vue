<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { useMessages } from '../../i18n'
import { useTheme } from '../../composables/useTheme'
import { Theme } from '../../../bindings/json-inspector/internal/domain'

const { t } = useMessages()
const { theme, setTheme } = useTheme()

// The handoff's order: light, dark, system. A segment holds a message key, not its words — the words
// are looked up as the switch is drawn, so they follow the language without a reload.
const OPTIONS: { value: Theme; icon: string; label: string }[] = [
  { value: Theme.ThemeLight, icon: 'sun', label: 'theme.light' },
  { value: Theme.ThemeDark, icon: 'moon', label: 'theme.dark' },
  { value: Theme.ThemeSystem, icon: 'monitor', label: 'theme.system' },
]

const rootEl = ref<HTMLElement | null>(null)

const activeIndex = computed(() => OPTIONS.findIndex((o) => o.value === theme.value))

const options = () => Array.from(rootEl.value?.querySelectorAll<HTMLElement>('.theme-option') ?? [])

// The pill is an ordinary element: it is on its segment the moment the choice is made, and the
// cross-fade over the change shows the same window in the other palette.
const indicatorStyle = computed(() => ({
  transform: `translateX(calc(${activeIndex.value} * (100% + 2px)))`,
}))

function pick(value: Theme) {
  if (value !== theme.value) setTheme(value)
}

// Chromium hands the hit test to the transition while one runs, so for those two hundred milliseconds
// pointer events land on `<html>` and the button never hears about them. The switch is the one control
// the user may need in that window — they just used it — so a click that was taken from it is answered
// here, by the segment under the pointer. Only that event: everything else keeps its own target.
function onHijackedPointerDown(e: PointerEvent) {
  if (e.target !== document.documentElement) return
  const at = options().findIndex((o) => {
    const r = o.getBoundingClientRect()
    return e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom
  })
  if (at !== -1) pick(OPTIONS[at].value)
}

onMounted(() => document.addEventListener('pointerdown', onHijackedPointerDown, true))
onBeforeUnmount(() => document.removeEventListener('pointerdown', onHijackedPointerDown, true))
</script>

<template>
  <div ref="rootEl" class="theme-switch" :title="t('theme.title')">
    <span class="theme-indicator" :style="indicatorStyle"></span>
    <button
      v-for="(o, i) in OPTIONS"
      :key="o.value"
      class="theme-option"
      :class="{ active: i === activeIndex }"
      :title="t(o.label)"
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

/* One element that holds the active segment, as in the auth panel. It carries no transition of its
   own on purpose: the window is snapshotted a frame after the click, and a pill still travelling
   would be caught in that snapshot and would jump the rest of the way once the fade is over. */
.theme-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 inline-flex items-center justify-center bg-bg-panel rounded-[5px];
  width: 24px;
  box-shadow: var(--shadow-btn);
}

.theme-option {
  @apply relative w-6 h-[22px] flex-none inline-flex items-center justify-center border-none bg-transparent rounded-[5px] text-text-tertiary cursor-pointer;
  transition: color 0.18s ease, transform 0.1s ease;
  --wails-draggable: no-drag;
}

.theme-option:hover,
.theme-option.active {
  @apply text-text;
}

/* The click reads as answered before the fade has begun. */
.theme-option:active {
  transform: scale(0.86);
}
</style>
