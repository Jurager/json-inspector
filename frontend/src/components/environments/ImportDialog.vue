<script setup lang="ts">
import { DialogTitle } from 'reka-ui'
import Dialog from '../ui/dialog/Dialog.vue'
import { Checkbox } from '../ui/checkbox'
import { useMessages } from '../../i18n'
import { Button } from '../ui/button'
import type { ImportChoice } from '../../stores/environments'

const { t } = useMessages()

const props = defineProps<{
  entries: ImportChoice[]
  existingNames: Set<string>
  targetName: string
  count: number
  open: boolean
}>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'apply'): void }>()
</script>

<template>
  <Dialog
    :open="props.open"
    class="import-panel"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <DialogTitle class="import-title">
      {{ t('environments.importTitle', { name: props.targetName }) }}
    </DialogTitle>
    <div class="import-body">
      <div v-for="e in props.entries" :key="e.name" class="import-row">
        <Checkbox
          :model-value="e.secret"
          tone="secret"
          :title="e.secret ? t('environments.importAsSecret') : t('environments.importAsVariable')"
          @update:model-value="(v: any) => (e.secret = Boolean(v))"
        />
        <span class="import-name mono">{{ e.name }}</span>
        <span class="import-value mono" :class="{ masked: e.secret }">{{ e.secret ? '••••' : e.value }}</span>
        <span v-if="props.existingNames.has(e.name)" class="import-conflict">
          <button class="conflict-btn" :class="{ on: e.mode === 'skip' }" @click="e.mode = 'skip'">{{ t('environments.skip') }}</button>
          <button class="conflict-btn" :class="{ on: e.mode === 'replace' }" @click="e.mode = 'replace'">{{ t('environments.replace') }}</button>
        </span>
        <span v-else class="import-new">{{ t('environments.newVar') }}</span>
      </div>
      <div v-if="props.entries.length === 0" class="import-empty">
        {{ t('environments.noKeyValue') }}
      </div>
    </div>
    <div class="import-actions">
      <span class="import-summary">{{ t('environments.willWrite', { n: props.count }) }}</span>
      <Button @click="emit('cancel')">{{ t('common.cancel') }}</Button>
      <Button variant="primary" :disabled="props.count === 0" @click="emit('apply')">{{ t('environments.import') }}</Button>
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
  @apply flex-1 text-xs text-text-tertiary;
}
</style>
