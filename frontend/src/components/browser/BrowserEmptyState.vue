<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { App as Backend } from '../../../bindings/json-inspector'
import { Browser } from '@wailsio/runtime'
import { useRequestsStore } from '../../stores/requests'
import { Button } from '../ui/button'

const store = useRequestsStore()

const port = ref('')

onMounted(async () => {
  try {
    port.value = String(await Backend.BridgePort())
  } catch {
    // Runtime not ready yet.
  }
})

const statusText = computed(() => {
  const p = port.value || '…'
  return store.capture.connected
    ? `Расширение подключено · порт ${p} слушает`
    : `Расширение не найдено · порт ${p} слушает`
})

function openInstructions() {
  Browser.OpenURL('https://github.com/Jurager/json-inspector')
}
</script>

<template>
  <div class="browser-empty">
    <div class="empty-box">
      <div class="empty-head">
        <div class="empty-title">Подключите расширение, чтобы видеть запросы браузера</div>
        <div class="empty-subtitle">Перехваченные запросы появятся здесь автоматически.</div>
      </div>

      <div class="steps">
        <div class="step">
          <span class="step-num">1</span>
          <span class="step-text">Установите расширение из папки <span class="step-code">extension/</span> в режиме разработчика.</span>
        </div>
        <div class="step">
          <span class="step-num">2</span>
          <span class="step-text">Откройте нужную вкладку и включите «Перехватывать эту вкладку» в попапе.</span>
        </div>
        <div class="step">
          <span class="step-num">3</span>
          <span class="step-text">Вернитесь сюда — вкладка появится в списке слева как отдельная группа.</span>
        </div>
      </div>

      <div class="empty-actions">
        <Button variant="primary" @click="openInstructions">Открыть инструкцию</Button>
        <span class="status-line">
          <span class="dot" :class="store.capture.connected ? 'dot-green' : 'dot-orange'"></span>
          <span>{{ statusText }}</span>
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.browser-empty {
  @apply flex items-center justify-center p-6;
}

.empty-box {
  @apply w-[560px] max-w-full flex flex-col gap-4.5;
}

.empty-head {
  @apply flex flex-col gap-1.5;
}

.empty-title {
  @apply text-[17px] font-semibold;
}

.empty-subtitle {
  @apply text-[13px] text-text-secondary leading-relaxed;
}

.steps {
  @apply flex flex-col gap-2.5;
}

.step {
  @apply flex items-baseline gap-2.5;
}

.step-num {
  @apply flex-none w-[18px] h-[18px] rounded-full bg-bg-inset text-text-secondary text-xs font-semibold inline-flex items-center justify-center;
}

.step-text {
  @apply text-[13px] leading-relaxed;
}

.step-code {
  @apply text-xs px-1.5 py-0.5 rounded bg-bg-inset;
  font-family: var(--mono);
}

.empty-actions {
  @apply flex items-center gap-3;
}

.status-line {
  @apply inline-flex items-center gap-1.5 text-xs text-text-secondary;
}

.dot {
  @apply w-[7px] h-[7px] rounded-full flex-none;
}

.dot-orange {
  background: var(--orange);
}

.dot-green {
  background: var(--green);
}
</style>
