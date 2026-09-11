<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import Icon from './Icon.vue'
import { useEnvironmentsStore } from '../stores/environments'
import { shortcut } from '../lib/platform'

const store = useEnvironmentsStore()

const emit = defineEmits<{ (e: 'close'): void }>()

const editHint = shortcut('E')

// Only the four colours the model allows, mapped onto existing tokens — the
// chip and the sheet read the same dot from here.
const DOT_COLORS: Record<string, string> = {
  green: 'var(--green)',
  orange: 'var(--orange)',
  red: 'var(--red)',
  purple: 'var(--purple)',
}

function dotStyle(color?: string) {
  return { background: DOT_COLORS[color ?? 'green'] ?? 'var(--green)' }
}

// Picking is a transient act, so the menu closes on it — the chip in the
// titlebar is what confirms the choice landed.
function choose(id: string | null) {
  store.setActive(id)
  emit('close')
}

function edit() {
  store.openSheet()
  emit('close')
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="menu env-menu">
    <div class="env-menu-head">Окружение</div>

    <button
      v-for="env in store.environments"
      :key="env.id"
      class="env-row"
      :class="{ active: env.id === store.activeId }"
      @click="choose(env.id)"
    >
      <span class="env-row-mark">
        <Icon v-if="env.id === store.activeId" name="check" :size="12" />
      </span>
      <span class="env-row-dot" :style="dotStyle(env.color)"></span>
      <span class="env-row-name">{{ env.name }}</span>
      <span v-if="env.readonly" class="env-badge">только чтение</span>
      <span v-else class="env-count">{{ env.vars.length }} перем.</span>
    </button>

    <button class="env-row" :class="{ active: store.activeId === null }" @click="choose(null)">
      <span class="env-row-mark">
        <Icon v-if="store.activeId === null" name="check" :size="12" />
      </span>
      <span class="env-row-name none">Без окружения</span>
    </button>

    <div class="env-divider"></div>

    <button class="env-row" @click="edit">
      <span class="env-row-mark"><Icon name="list" :size="13" /></span>
      <span class="env-row-name">Редактировать переменные…</span>
      <span class="env-hint mono">{{ editHint }}</span>
    </button>
  </div>
</template>

<style scoped>
@reference "../style.css";

.env-menu {
  @apply absolute right-0 w-[280px] p-1.5 rounded-[11px];
  top: calc(100% + 6px);
}

.env-menu-head {
  @apply pt-1.5 px-2.5 pb-1 text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
  font-family: var(--mono);
}

.env-row {
  @apply flex items-center gap-2 w-full py-[7px] px-2.5 border-none rounded-md bg-transparent text-text text-[13px] text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.env-row:hover {
  @apply bg-bg-hover;
}

.env-row.active {
  @apply bg-accent-soft font-semibold;
}

/* Fixed slot instead of per-state padding, so the names of active and inactive
   rows line up without hardcoding "30px of room for a tick". */
.env-row-mark {
  @apply flex-none w-3 inline-flex items-center justify-center text-accent;
}

.env-row-dot {
  @apply flex-none w-[7px] h-[7px] rounded-full;
}

.env-row-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.env-row-name.none {
  @apply text-text-secondary;
}

.env-count {
  @apply flex-none text-[10.5px] text-text-tertiary;
  font-family: var(--mono);
}

.env-badge {
  @apply flex-none text-red bg-red-soft text-[10px] font-semibold py-px px-[5px] rounded-sm;
}

.env-divider {
  @apply h-px mx-2 my-[5px] bg-border;
}

.env-hint {
  @apply flex-none text-[10.5px] text-text-tertiary;
}
</style>
