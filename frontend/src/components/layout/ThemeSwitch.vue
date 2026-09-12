<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { useTheme, type Theme } from '../../composables/useTheme'

const { theme, setTheme } = useTheme()

// The handoff's order: light, dark, system.
const OPTIONS: { value: Theme; icon: string; title: string }[] = [
  { value: 'light', icon: 'sun', title: 'Светлая' },
  { value: 'dark', icon: 'moon', title: 'Тёмная' },
  { value: 'system', icon: 'monitor', title: 'Системная' },
]

// The pill lands before the theme changes: inside the view transition the window is a
// frozen snapshot, so a pill caught mid-flight would stay caught for the whole wipe.
const pending = ref<Theme | null>(null)
const indicator = ref<HTMLElement>()
const activeIndex = computed(() => OPTIONS.findIndex((o) => o.value === theme.value))
const shownIndex = computed(() => {
  const next = pending.value ? OPTIONS.findIndex((o) => o.value === pending.value) : -1
  return next === -1 ? activeIndex.value : next
})
const indicatorStyle = computed(() => ({
  transform: `translateX(calc(${shownIndex.value} * (100% + 2px)))`,
}))

let timer: number | undefined

function commit() {
  if (!pending.value) return
  clearTimeout(timer)
  // `transitionend` is the cue; polling covers a missed event, and waiting for the pill
  // itself means no assumption about how long its transition takes.
  if (indicator.value?.getAnimations().some((a) => a.playState === 'running')) {
    timer = window.setTimeout(commit, 40)
    return
  }
  const next = pending.value
  pending.value = null
  setTheme(next)
}

function pick(value: Theme) {
  if (value === theme.value) return
  pending.value = value
  clearTimeout(timer)
  timer = window.setTimeout(commit, 60)
}
</script>

<template>
  <div class="theme-switch" title="Тема оформления">
    <span ref="indicator" class="theme-indicator" :style="indicatorStyle" @transitionend="commit"></span>
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

/* The moving segment, as in the auth panel: one element that slides, rather than a
   background hopping between buttons. */
.theme-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 bg-bg-panel rounded-[5px];
  width: 24px;
  transition: transform 0.18s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: var(--shadow-btn);
}

.theme-option {
  @apply relative w-6 h-[22px] flex-none inline-flex items-center justify-center border-none bg-transparent rounded-[5px] text-text-tertiary cursor-pointer;
  transition: color 0.18s ease;
  --wails-draggable: no-drag;
}

.theme-option:hover,
.theme-option.active {
  @apply text-text;
}
</style>
