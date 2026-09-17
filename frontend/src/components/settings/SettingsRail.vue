<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { SystemService } from '../../../bindings/json-inspector/internal/transport/wails'
import Icon from '../ui/Icon.vue'
import { CATEGORIES, metaKey, nameKey, type CategoryId } from './categories'
import { useUpdateCheck } from '../../composables/useUpdateCheck'
import { useMessages } from '../../i18n'
import { formatVersion } from '../../lib/format'

// The window's index: one row per category, each with the tile that tells them apart at a glance, and
// the version at the foot — the one line about the app that has no category of its own.
const props = defineProps<{ active: CategoryId }>()
const emit = defineEmits<{ (e: 'select', id: CategoryId): void }>()

const { t } = useMessages()
const { phase, latest } = useUpdateCheck()

const version = ref('')
const build = ref('')
const appName = ref('')

// The build, and how the app stands on updates: the second line is the same news the update window
// carries, at the size one line can hold. A check that has never run says nothing about releases
// rather than claiming the app is current.
const meta = computed(() => {
  const line = t('settings.build', { build: build.value || '—' })
  if (phase.value === 'available') {
    return `${line} · ${t('update.availableLinkShort', { version: formatVersion(latest.value) })}`
  }
  return phase.value === 'uptodate' ? `${line} · ${t('update.upToDate')}` : line
})

const versionLabel = computed(() =>
  [appName.value, formatVersion(version.value)].filter(Boolean).join(' ')
)

onMounted(async () => {
  // Cosmetic fields: a failed call leaves them blank, which is what a dev build shows anyway.
  try {
    version.value = (await SystemService.Version()) ?? ''
    build.value = (await SystemService.Build()) ?? ''
    appName.value = (await SystemService.Name()) ?? ''
  } catch {
    // Nothing to do: the rail is a list of categories either way.
  }
})
</script>

<template>
  <nav class="rail">
    <div class="rows">
      <button
        v-for="category in CATEGORIES"
        :key="category.id"
        type="button"
        class="row"
        :class="{ active: props.active === category.id }"
        @click="emit('select', category.id)"
      >
        <span class="tile" :style="{ background: category.tile }">
          <Icon :name="category.icon" :size="14" :stroke-width="1.9" />
        </span>
        <span class="lines">
          <span class="name">{{ t(nameKey(category.id)) }}</span>
          <span class="meta">{{ t(metaKey(category)) }}</span>
        </span>
      </button>
    </div>

    <div class="foot">
      <span class="foot-version">{{ versionLabel }}</span>
      <span class="foot-meta">{{ meta }}</span>
    </div>
  </nav>
</template>

<style scoped>
@reference "../../style.css";

.rail {
  @apply flex-none w-[236px] flex flex-col border-r;
  padding: 8px 8px 0;
  background: var(--bg-sidebar);
  border-color: var(--border);
}

.rows {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col;
}

.row {
  @apply flex items-center gap-2.5 text-left border-0 rounded-lg bg-transparent cursor-pointer;
  font: inherit;
  padding: 8px 9px;
  color: var(--text);
  --wails-draggable: no-drag;
}

.row:hover {
  @apply bg-bg-hover;
}

.row.active {
  @apply bg-accent-soft;
}

.row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

/* The tile is the category's colour with the glyph knocked out of it in white, as the drawing has it:
   the mark is what is recognised before the word under it is read. */
.tile {
  @apply inline-flex items-center justify-center flex-none w-[22px] h-[22px] rounded-md text-white;
}

.lines {
  @apply flex-1 min-w-0 flex flex-col;
  gap: 1px;
}

.name {
  @apply text-[13px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.row.active .name {
  @apply font-semibold;
}

.meta {
  @apply text-[11px] text-text-tertiary overflow-hidden text-ellipsis whitespace-nowrap;
}

/* The version is not a row: it is the one thing the rail says about the app rather than about where
   the user is in it, and the hairline is what keeps it out of the list. */
.foot {
  @apply flex-none flex flex-col;
  padding: 8px 9px;
  gap: 2px;
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.foot-version {
  @apply text-[11.5px] text-text-secondary;
}

.foot-meta {
  @apply text-[11px] text-text-tertiary;
}
</style>
