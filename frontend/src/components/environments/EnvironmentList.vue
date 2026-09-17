<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../ui/Icon.vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { GLOBALS_COLOR, initialOf, tintOf } from './palette'
import { useMessages } from '../../i18n'

// The rail: which set of variables the pane beside it is about. A row is a name, the colour it wears
// and a line saying what is in it; a new environment is made from the button underneath, which is the
// only thing this rail does besides choosing. While that button has been pressed the whole rail is
// out of the way, so the button is never on screen in a state of its own.
const emit = defineEmits<{ (e: 'select', id: string | null): void; (e: 'create'): void }>()

const envStore = useEnvironmentsStore()
const { t } = useMessages()

interface Row {
  id: string | null
  name: string
  color: string
  count: number
  readonly: boolean
  active: boolean
}

const rows = computed<Row[]>(() => [
  ...envStore.environments.map((e) => ({
    id: e.id as string | null,
    name: e.name,
    color: e.color ?? '',
    count: e.vars.length,
    readonly: e.readonly,
    active: e.id === envStore.activeId,
  })),
  // The globals are a row of the same list and not a section of their own: they are the scope that
  // applies everywhere, and a name to look at like any other.
  {
    id: null,
    name: t('environments.globals'),
    color: GLOBALS_COLOR,
    count: envStore.globals.length,
    readonly: false,
    active: false,
  },
])

// What the second line says: how much is in it, and then the one thing about it that is not a count
// — whether requests are being sent with it, or whether it is closed to editing.
function metaOf(row: Row): string {
  const count = t('counts.variables', row.count)
  if (row.active) return count + t('environments.activeMark')
  if (row.readonly) return count + t('environments.readonlyMark')
  return count
}
</script>

<template>
  <div class="sheet-side">
    <div class="side-rows">
      <button
        v-for="row in rows"
        :key="row.id ?? 'globals'"
        type="button"
        class="side-row"
        :class="{ active: row.id === envStore.editedEnvId }"
        @click="emit('select', row.id)"
      >
        <span class="avatar" :style="{ background: tintOf(row.color) }">{{ initialOf(row.name) }}</span>
        <span class="side-text">
          <span class="side-name">{{ row.name }}</span>
          <span class="side-meta">{{ metaOf(row) }}</span>
        </span>
        <Icon v-if="row.readonly" name="lock" :size="12" class="side-lock" />
      </button>
    </div>

    <div class="side-foot">
      <button type="button" class="side-new" @click="emit('create')">
        <Icon name="plus" :size="14" :stroke-width="2.2" />
        {{ t('environments.new') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-side {
  @apply flex-none w-[236px] flex flex-col p-2 pb-0 border-r;
  background: var(--glass-sheet-side);
  border-color: var(--border);
}

/* The rows scroll and the button does not: a workspace with twenty environments would otherwise
   push the way to make another one off the leaf. No gap between them — the drawing leaves none, and
   a gap turns one block of names into a list of separate things. */
.side-rows {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col;
}

.side-row {
  @apply flex items-center gap-2.5 w-full text-left border-0 rounded-lg
         bg-transparent cursor-pointer text-text;
  font-family: inherit;
  padding: 8px 9px;
  --wails-draggable: no-drag;
}

.side-row:hover {
  @apply bg-bg-hover;
}

.side-row.active {
  @apply bg-accent-soft;
}

/* The avatar letters the tab groups of the history panel wear, at the size this rail asks for. */
.avatar {
  @apply flex-none inline-flex items-center justify-center w-[22px] h-[22px] rounded-md
         text-white font-bold;
  font-size: 10.5px;
  /* The line box keeps room for the descenders a capital never reaches, so a single letter sits
     about half of that below the middle of the square. Padding at the foot shrinks the box the line
     is centred in, which lifts it by half the padding — and in em, so every size is the same. */
  padding-bottom: 0.09em;
}

.side-text {
  @apply flex-1 min-w-0 flex flex-col gap-px;
}

.side-name {
  @apply text-[13px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.side-row.active .side-name {
  @apply font-semibold;
}

.side-meta {
  @apply text-[11px] text-text-tertiary overflow-hidden text-ellipsis whitespace-nowrap;
}

.side-lock {
  @apply flex-none text-text-tertiary;
}

/* The same 48px bar as the panes' own feet, which is what keeps the button in it level with theirs:
   10px of air around a 28px button puts its centre 24 above the bottom, and a 48px bar centring what
   stands in it puts theirs 23.5 — half a pixel, which is a hairline nobody can see. */
.side-foot {
  @apply flex-none;
  padding: 10px 2px;
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.side-new {
  @apply flex items-center gap-[7px] h-7 px-2 border-0 rounded-md cursor-pointer
         bg-transparent text-accent;
  font-family: inherit;
  font-size: 12.5px;
  font-weight: 500;
}

.side-new:hover {
  @apply bg-accent-soft;
}
</style>
