<script setup lang="ts">
import { DialogTitle } from 'reka-ui'
import Dialog from '../ui/dialog/Dialog.vue'
import { Checkbox } from '../ui/checkbox'
import { Button } from '../ui/button'
import type { ImportChoice } from '../../stores/environments'

// One review pass over what was read from the file, then one button. The rows
// are edited in place — the array is the caller's, and only its items change.
const props = defineProps<{
  entries: ImportChoice[]
  existingNames: Set<string>
  targetName: string
  count: number
}>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'apply'): void }>()
</script>

<template>
  <!-- Its own header rather than the dialog's title: this one is a full-width
       bar with the list scrolling underneath it. Escape goes to the sheet's
       cascade, as in DeleteEnvDialog. -->
  <Dialog
    :open="true"
    class="import-panel"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <DialogTitle class="import-title">
      Импорт .env → {{ props.targetName }}
    </DialogTitle>
    <div class="import-body">
      <div v-for="e in props.entries" :key="e.name" class="import-row">
        <Checkbox
          :model-value="e.secret"
          tone="secret"
          :title="e.secret ? 'Импортировать как секрет' : 'Импортировать как обычную переменную'"
          @update:model-value="(v) => (e.secret = Boolean(v))"
        />
        <span class="import-name mono">{{ e.name }}</span>
        <span class="import-value mono" :class="{ masked: e.secret }">{{ e.secret ? '••••' : e.value }}</span>
        <span v-if="props.existingNames.has(e.name)" class="import-conflict">
          <button class="conflict-btn" :class="{ on: e.mode === 'skip' }" @click="e.mode = 'skip'">пропустить</button>
          <button class="conflict-btn" :class="{ on: e.mode === 'replace' }" @click="e.mode = 'replace'">заменить</button>
        </span>
        <span v-else class="import-new">новая</span>
      </div>
      <div v-if="props.entries.length === 0" class="import-empty">
        В файле не нашлось строк вида KEY=value
      </div>
    </div>
    <div class="import-actions">
      <span class="import-summary">Будет записано: {{ props.count }}</span>
      <Button @click="emit('cancel')">Отмена</Button>
      <Button variant="primary" :disabled="props.count === 0" @click="emit('apply')">Импортировать</Button>
    </div>
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

.import-title {
  @apply flex-none px-3.5 py-3 text-[13px] font-semibold border-b border-border;
}

.import-body {
  @apply flex-1 min-h-0 overflow-auto py-1;
}

.import-row {
  @apply grid items-center gap-2.5 px-3.5 py-1.5;
  grid-template-columns: 20px 180px minmax(0, 1fr) auto;
}

.import-name {
  @apply text-[12.5px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.import-value {
  @apply text-[12.5px] text-text-secondary overflow-hidden text-ellipsis whitespace-nowrap;
}

.import-value.masked {
  @apply text-text-tertiary;
}

.import-new {
  @apply text-[11px] text-text-tertiary px-2;
}

.import-conflict {
  @apply inline-flex gap-0.5 p-0.5 rounded-md;
  background: var(--bg-inset);
}

.conflict-btn {
  @apply text-[11px] py-0.5 px-2 border-none rounded-md bg-transparent text-text-secondary cursor-pointer;
  font: inherit;
}

.conflict-btn.on {
  @apply bg-bg-panel text-text font-medium;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.import-empty {
  @apply px-3.5 py-3 text-xs text-text-tertiary;
}

.import-actions {
  @apply flex-none flex items-center gap-2 px-3.5 py-2.5 border-t border-border;
}

.import-summary {
  @apply flex-1 text-[11.5px] text-text-tertiary;
}
</style>
