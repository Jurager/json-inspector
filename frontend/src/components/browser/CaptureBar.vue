<script setup lang="ts">
import { computed } from 'vue'
import { Button } from '../ui/button'
import { useMessages } from '../../i18n'
import { useRequestsStore } from '../../stores/requests'
import { BridgeService } from '../../../bindings/json-inspector/internal/transport/wails'

const store = useRequestsStore()

const { t } = useMessages()

const sourceLabel = computed(() => {
  const title = store.browserSelected?.tabTitle
  return title ? t('browser.sourceTab', { title }) : t('browser.sourceBrowser')
})

const recording = computed(() => store.capture.recording)

async function toggleCapture() {
  if (recording.value) await BridgeService.PauseCapture()
  else await BridgeService.ResumeCapture()
}
</script>

<template>
  <div class="capture-bar">
    <span class="capture-source">{{ sourceLabel }}</span>
    <span class="capture-spacer"></span>
    <span class="capture-hint">{{ t('browser.readonly') }}</span>
    <Button
      size="sm"
      :title="recording ? t('browser.stopAll') : t('browser.restoreAll')"
      @click="toggleCapture"
    >
      {{ recording ? t('browser.pause') : t('browser.resume') }}
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
  @apply text-xs text-text-secondary min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.capture-spacer {
  @apply flex-1;
}

.capture-hint {
  @apply text-xs text-text-tertiary whitespace-nowrap;
}

</style>
