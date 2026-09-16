<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { SystemService } from '../../bindings/json-inspector/internal/transport/wails'
import { usePlatform } from '../composables/usePlatform'
import UpdateCheck from '../components/update/UpdateCheck.vue'
import { useMessages } from '../i18n'
import { formatVersion } from '../lib/format'
import logoUrl from '../assets/logo.svg'

const { t } = useMessages()
const { customTitlebar } = usePlatform()

const version = ref('')
const build = ref('')
const appName = ref('')
const year = new Date().getFullYear()

const versionLabel = computed(() => {
  const v = formatVersion(version.value)
  return build.value ? `${v} (${build.value})` : v
})

onMounted(async () => {
  // The title bar this window draws is ours, but the taskbar reads the platform's name for it — and
  // that one is a word, so it comes from the catalogue rather than from Go.
  void Window.SetTitle(t('about.title'))
  // Cosmetic fields: a failed call just leaves them blank.
  try {
    version.value = (await SystemService.Version()) ?? ''
    build.value = (await SystemService.Build()) ?? ''
  } catch {}
  try {
    appName.value = (await SystemService.Name()) ?? ''
  } catch {}
})
</script>

<template>
  <div class="about-window" :class="{ 'about-window-mac': !customTitlebar }">
    <header v-if="customTitlebar" class="about-bar">
      <span class="about-bar-title">{{ t('about.title') }}</span>
      <button class="cap-btn cap-close" :title="t('common.close')" @click="Window.Close()">
        <span class="cap-icon cap-icon-close">
          <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
          <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
        </span>
      </button>
    </header>

    <div class="about-body">
      <img class="about-icon" :src="logoUrl" :alt="appName" draggable="false" />
      <div class="about-name">{{ appName }}</div>
      <div class="about-version">{{ t('about.version', { version: versionLabel || '…' }) }}</div>

      <UpdateCheck />

      <div class="about-divider"></div>
      <div class="about-copy">
        {{ t('about.copyright', { year, name: appName || 'JSON Inspector' }) }}<br />
        {{ t('about.rights') }}
      </div>
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

/* Where the bar is ours the title reads from the left, as the main window's does and as the platform
   puts it; the design centres it because it draws macOS, where this bar is not drawn at all and the
   system writes the title itself. The inset is the main titlebar's own `px-3`, so the two rows start
   at the same place. */
.about-bar {
  @apply relative flex-none h-13 flex items-center justify-start px-3;
  border-bottom: 1px solid var(--glass-chrome-border);
  background: var(--glass-chrome);
  backdrop-filter: var(--blur-chrome);
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

.about-divider {
  @apply w-full h-px mt-4 mb-3.5;
  background: var(--border);
}

.about-copy {
  @apply text-[11px] leading-[1.6] text-text-tertiary;
}
</style>
