<script setup lang="ts">
import { computed } from 'vue'
import { Button } from '../ui/button'
import { useRequestsStore } from '../../stores/requests'
import { App as Backend } from '../../../bindings/json-inspector'

const store = useRequestsStore()

// The source is the browser tab the selected request came from.
const sourceLabel = computed(() => {
  const t = store.browserSelected?.tabTitle
  return t ? `Источник: вкладка «${t}»` : 'Источник: браузер'
})

// One control, two states: with recording on it stops every tab, and once
// stopped it puts back the tabs that were being recorded. The label follows the
// live capture state the extension reports rather than what was last clicked,
// so it can't drift out of sync with reality.
const recording = computed(() => store.capture.recording)

async function toggleCapture() {
  if (recording.value) await Backend.PauseCapture()
  else await Backend.ResumeCapture()
}
</script>

<template>
  <div class="capture-bar">
    <span class="capture-source">{{ sourceLabel }}</span>
    <span class="capture-spacer"></span>
    <span class="capture-hint">Только чтение — запросы уже выполнены</span>
    <Button
      size="sm"
      :title="recording ? 'Остановить перехват на всех вкладках' : 'Вернуть перехват на прежние вкладки'"
      @click="toggleCapture"
    >
      {{ recording ? 'Приостановить перехват' : 'Возобновить перехват' }}
    </Button>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.capture-bar {
  @apply flex-none flex items-center gap-2 h-[38px] px-4 border-b border-border;
  background: var(--bg-inset);
}

.capture-source {
  @apply text-[11.5px] text-text-secondary min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.capture-spacer {
  @apply flex-1;
}

.capture-hint {
  @apply text-[11.5px] text-text-tertiary whitespace-nowrap;
}

</style>
