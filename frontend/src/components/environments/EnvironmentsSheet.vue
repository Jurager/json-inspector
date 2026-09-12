<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { shortcut } from '../../lib/platform'
import { useSheetNotice } from '../../composables/useSheetNotice'
import EnvironmentList from './EnvironmentList.vue'
import { Button } from '../ui/button'
import VariablesTable from './VariablesTable.vue'

const emit = defineEmits<{ (e: 'close'): void }>()

const editHint = shortcut('E')

const listRef = ref<InstanceType<typeof EnvironmentList> | null>(null)
const tableRef = ref<InstanceType<typeof VariablesTable> | null>(null)

const { clearNotice } = useSheetNotice()

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (tableRef.value?.cancelTop()) return
  if (listRef.value?.cancelTop()) return
  emit('close')
}

onMounted(() => {
  clearNotice()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="sheet-overlay">
    <div class="sheet">
      <div class="sheet-head">
        <span class="sheet-title">Переменные окружения</span>
        <span class="sheet-hint mono">{{ editHint }}</span>
        <Button @click="emit('close')">Готово</Button>
      </div>

      <div class="sheet-body">
        <EnvironmentList ref="listRef" />
        <VariablesTable ref="tableRef" />
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-overlay {
  @apply fixed inset-0 z-1500 flex items-center justify-center;
  background: rgba(0, 0, 0, 0.25);
}

.sheet {
  @apply flex flex-col rounded-xl overflow-hidden w-[1040px] max-w-[95vw];
  max-height: calc(100vh - 72px);
  background: var(--bg-panel);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.18), 0 0 0 1px var(--border);
}

.sheet-head {
  @apply flex-none flex items-center gap-3 h-[46px] px-3.5 border-b border-border;
  background: var(--bg-sidebar);
}

.sheet-title {
  @apply text-[13px] font-semibold;
}

.sheet-hint {
  @apply flex-1 text-[10.5px] text-text-tertiary;
}


.sheet-body {
  @apply flex h-[548px] min-h-0;
}
</style>
