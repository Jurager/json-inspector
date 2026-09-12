<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { App as Backend } from '../../bindings/json-inspector'
import { usePlatform } from '../composables/usePlatform'
import { useUpdateCheck } from '../composables/useUpdateCheck'
import { Button } from '../components/ui/button'
import Icon from '../components/ui/Icon.vue'
import { formatCheckedAt, formatVersion } from '../lib/format'
import logoUrl from '../assets/logo.svg'

const { customTitlebar } = usePlatform()
const { phase, latest, checkedAt, error, check, install } = useUpdateCheck()

const version = ref('')
const build = ref('')
const appName = ref('')
const year = new Date().getFullYear()

const versionLabel = computed(() => {
  const v = formatVersion(version.value)
  return build.value ? `${v} (${build.value})` : v
})

const buttonLabel = computed(() => {
  switch (phase.value) {
    case 'checking':
      return 'Проверяем…'
    case 'installing':
      return 'Обновление…'
    case 'available':
      return 'Обновить'
    case 'uptodate':
    case 'error':
      return 'Проверить снова'
    default:
      return 'Проверить обновления'
  }
})

const busy = computed(() => phase.value === 'checking' || phase.value === 'installing')

const act = computed(() => (phase.value === 'available' ? install : check))

onMounted(async () => {
  // Cosmetic fields: a failed call just leaves them blank.
  try {
    version.value = (await Backend.Version()) ?? ''
    build.value = (await Backend.Build()) ?? ''
  } catch {}
  try {
    appName.value = (await Backend.Name()) ?? ''
  } catch {}
})
</script>

<template>
  <div class="about-window" :class="{ 'about-window-mac': !customTitlebar }">
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
      <div class="about-version">Версия {{ versionLabel || '…' }}</div>

      <Button class="about-check" variant="primary" size="lg" :disabled="busy" @click="act">
        <span v-if="busy" class="about-spinner"></span>
        {{ buttonLabel }}
      </Button>

      <div class="about-status">
        <template v-if="phase === 'uptodate'">
          <span class="about-ok">
            <Icon name="check" :size="13" :stroke-width="2.4" />
            <span>Установлена последняя версия</span>
          </span>
        </template>
        <template v-else-if="phase === 'checking'">Проверяем обновления…</template>
        <template v-else-if="phase === 'installing'">Скачиваем и проверяем…</template>
        <template v-else-if="phase === 'available'">Доступна версия {{ formatVersion(latest) }}</template>
        <template v-else-if="phase === 'error'">
          <span class="about-error">{{ error }}</span>
        </template>
        <template v-else>Последняя проверка: {{ formatCheckedAt(checkedAt) }}</template>
      </div>

      <div class="about-divider"></div>
      <div class="about-copy">© {{ year }} {{ appName || 'JSON Inspector' }}.<br />Все права защищены.</div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.about-window {
  @apply h-full flex flex-col bg-bg-panel text-text select-none;
}

/* macOS hides its title bar inside the window, so the content starts below it; the number
   matches `InvisibleTitleBarHeight` in window.go. */
.about-window-mac .about-body {
  padding-top: 50px;
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

/* The mock's own 6px above the icon sit under an empty 40px strip; where we draw a bar with a
   border line instead, the same 6px reads as cramped, so the sides get a little more air. */
.about-body {
  @apply flex-1 flex flex-col items-center justify-center px-[30px] pt-[22px] pb-9 text-center gap-[3px];
}

.about-icon {
  @apply w-26 h-26 mb-4;
  filter: drop-shadow(0 12px 20px rgba(32, 184, 137, 0.4));
  -webkit-user-drag: none;
  user-drag: none;
}

.about-name {
  @apply text-[19px] font-bold tracking-[-0.015em];
}

.about-version {
  @apply font-mono text-xs text-text-tertiary tabular-nums mb-[18px];
}

.about-check {
  @apply min-w-[186px];
}

/* White on the accent button, where the app's own `.spinner` (accent ring) would vanish. */
.about-spinner {
  @apply w-[13px] h-[13px] rounded-full inline-block mr-1.5;
  border: 2px solid rgba(255, 255, 255, 0.45);
  border-top-color: #fff;
  animation: spin 0.7s linear infinite;
}

.about-status {
  @apply h-7 mt-3 flex items-center justify-center text-xs text-text-tertiary;
}

.about-ok {
  @apply inline-flex items-center gap-[5px] text-green font-medium;
}

.about-error {
  @apply text-red;
}

.about-divider {
  @apply w-full h-px mt-4 mb-3.5;
  background: var(--border);
}

.about-copy {
  @apply text-[11px] leading-[1.6] text-text-tertiary;
}
</style>
