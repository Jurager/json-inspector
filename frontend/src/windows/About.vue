<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { App as Backend } from '../../bindings/json-inspector'
import { usePlatform } from '../composables/usePlatform'
import logoUrl from '../assets/logo.svg'

const { customTitlebar } = usePlatform()

const version = ref('')
const appName = ref('')
const year = new Date().getFullYear()

onMounted(async () => {
  try {
    version.value = (await Backend.Version()) ?? ''
  } catch {
    // Both fields are cosmetic: a failed call just leaves the field blank.
    version.value = ''
  }
  try {
    appName.value = (await Backend.Name()) ?? ''
  } catch {}
})
</script>

<template>
  <div class="about-window">
    <header v-if="customTitlebar" class="about-bar">
      <span class="about-bar-title">О программе</span>
      <button class="cap-btn cap-close" title="Закрыть" @click="Window.Close()">
        <span class="cap-icon cap-icon-close">
          <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
          <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
        </span>
      </button>
    </header>

    <div class="about-body">
      <img class="about-icon" :src="logoUrl" :alt="appName" draggable="false" />
      <div class="about-name">{{ appName }}</div>
      <div class="about-version">Версия {{ version || '…' }}</div>
      <p class="about-desc">
        Инструмент для работы с JSON и JSON:API: подстановка переменных окружения,
        карта схемы и перехват запросов из браузера.
      </p>
      <div class="about-tech">Go · Wails v3 · Vue</div>
      <div class="about-copy">© {{ year }} Yuri Gerasimov</div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.about-window {
  @apply h-full flex flex-col bg-bg-panel text-text select-none;
}

.about-bar {
  @apply relative flex-none h-13 flex items-center justify-center bg-bg-sidebar border-b border-border;
  --wails-draggable: drag;
}

.about-bar-title {
  @apply text-[13px] font-medium text-text-secondary;
}

.about-bar .cap-btn {
  @apply absolute right-0;
}

.about-body {
  @apply flex-1 flex flex-col items-center justify-center gap-1.5 px-6 text-center;
}

.about-icon {
  @apply w-22 h-22 mb-2.5;
  -webkit-user-drag: none;
  user-drag: none;
}

.about-name {
  @apply text-[15px] font-semibold;
}

.about-version {
  @apply text-xs text-text-secondary tabular-nums;
}

.about-desc {
  @apply mt-3 mb-0 text-[12.5px] leading-[1.5] text-text-secondary max-w-[320px];
}

.about-tech {
  @apply mt-2 text-[11px] text-text-tertiary;
}

.about-copy {
  @apply mt-5 text-[11px] text-text-tertiary;
}
</style>
