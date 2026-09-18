<script setup lang="ts">
import { computed } from 'vue'
import { Sheet, SheetRow } from '../ui/sheet'
import { RecordsService } from '../../../bindings/json-inspector/internal/transport/wails'
import { formatBytes, useMessages } from '../../i18n'

// The confirmation before a history goes: what is about to be thrown away, in the two numbers that
// say how much of it there is. It is the sheet the environments window raises for a delete, which is
// why the row that states the cost carries the fill of something being kept until the very last.
const props = defineProps<{ count: number; bytes: number }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'cleared'): void }>()

const { t } = useMessages()

const cleared = computed(() => t('counts.requests', props.count))

async function clear() {
  try {
    await RecordsService.ClearAll()
  } finally {
    // Either way the panel has to read its numbers again: a call that failed leaves the history as it
    // was, and one that worked leaves none of it.
    emit('cleared')
  }
}
</script>

<template>
  <Sheet
    :title="t('settings.clearHistory')"
    :sub="t('settings.clearHistorySub')"
    :cancel="t('common.cancel')"
    :action="t('settings.clearAction')"
    @close="emit('close')"
    @cancel="emit('close')"
    @action="clear"
  >
    <div class="rows">
      <SheetRow
        :label="cleared"
        :note="t('settings.clearSize', { size: formatBytes(props.bytes) })"
        :tag="t('settings.clearTag')"
        tone="drop"
      />
    </div>
  </Sheet>
</template>

<style scoped>
@reference "../../style.css";

.rows {
  @apply flex flex-col gap-0.5;
}
</style>
