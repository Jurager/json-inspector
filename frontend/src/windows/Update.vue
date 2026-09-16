<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { SystemService } from '../../bindings/json-inspector/internal/transport/wails'
import { usePlatform } from '../composables/usePlatform'
import { useUpdateCheck } from '../composables/useUpdateCheck'
import { Button } from '../components/ui/button'
import { useMessages } from '../i18n'
import { formatVersion } from '../lib/format'
import logoUrl from '../assets/logo.svg'

const { t } = useMessages()
const { customTitlebar } = usePlatform()
const { phase, latest, notes, error, install, skip } = useUpdateCheck()

const appName = ref('')
const version = ref('')
const build = ref('')

const versionLabel = computed(() => {
  const v = formatVersion(version.value)
  return build.value ? `${v} (${build.value})` : v || '…'
})

const busy = computed(() => phase.value === 'installing')

onMounted(async () => {
  // The title bar this window draws is ours, but the taskbar reads the platform's name for it — and
  // that one is a sentence in a language, so it comes from the catalogue rather than from Go.
  void Window.SetTitle(t('update.title', { app: appName.value || 'JSON Inspector' }))
  // Cosmetic fields: a failed call just leaves them blank.
  try {
    appName.value = (await SystemService.Name()) ?? ''
  } catch {}
  try {
    version.value = (await SystemService.Version()) ?? ''
    build.value = (await SystemService.Build()) ?? ''
  } catch {}
})

// Skipping is about the version on screen, and the backend is what knows which one that is. Closing
// afterwards is the whole point of the button: the user asked not to be told about it again.
async function onSkip() {
  await skip()
  Window.Close()
}
</script>

<template>
  <div class="update-window" :class="{ 'update-window-mac': !customTitlebar }">
    <header v-if="customTitlebar" class="update-bar">
      <span class="update-bar-title">{{ t('update.title', { app: appName || 'JSON Inspector' }) }}</span>
      <button class="cap-btn cap-close" :title="t('common.close')" @click="Window.Close()">
        <span class="cap-icon cap-icon-close">
          <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
          <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
        </span>
      </button>
    </header>

    <div class="update-body">
      <div class="update-head">
        <img class="update-icon" :src="logoUrl" :alt="appName" draggable="false" />
        <div class="update-heading">
          <div class="update-title">
            {{ latest ? t('update.available', { version: formatVersion(latest) }) : t('update.none') }}
          </div>
          <div class="update-subtitle">
            {{ t('update.current', { app: appName || 'JSON Inspector', version: versionLabel }) }}
          </div>
        </div>
      </div>

      <div class="update-whats-new">
        <div class="update-label">{{ t('update.whatsNew') }}</div>
        <div class="update-notes">
          <div v-for="(note, index) in notes" :key="index" class="update-note">
            <span class="update-note-dot"></span>
            <span>{{ note }}</span>
          </div>
          <!-- A release whose body was left empty, and the running build when nothing is offered:
               both are real states, and an empty box would read as a page that failed to load. -->
          <p v-if="!notes.length" class="update-empty">{{ t('update.noNotes') }}</p>
        </div>
      </div>

      <div class="update-actions">
        <button class="update-skip" :disabled="busy" @click="onSkip">
          {{ t('update.skip') }}
        </button>
        <div class="update-actions-right">
          <Button variant="outline" size="lg" :disabled="busy" @click="Window.Close()">
            {{ t('update.remindLater') }}
          </Button>
          <Button variant="primary" size="lg" :disabled="busy || !latest" @click="install">
            {{ busy ? t('update.installing') : t('update.installAndRestart') }}
          </Button>
        </div>
      </div>

      <p v-if="error" class="update-error">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.update-window {
  @apply h-full flex flex-col bg-bg-panel text-text select-none;
}

/* macOS hides its title bar inside the window, so the content starts below it; the number matches
   `InvisibleTitleBarHeight` in window.go. */
.update-window-mac .update-body {
  padding-top: 50px;
}

/* Left, like the main window's titlebar and like the platform: the design centres it because it draws
   macOS, where the system draws the title and this bar does not exist. */
.update-bar {
  @apply relative flex-none h-13 flex items-center justify-start px-3;
  border-bottom: 1px solid var(--glass-chrome-border);
  background: var(--glass-chrome);
  backdrop-filter: var(--blur-chrome);
  --wails-draggable: drag;
}

.update-bar-title {
  @apply text-[13px] font-medium text-text-secondary;
}

.update-bar .cap-btn {
  @apply absolute right-0;
}

.update-body {
  @apply flex-1 min-h-0 flex flex-col gap-[18px] px-[30px] pt-7 pb-6;
}

.update-head {
  @apply flex items-start gap-4 flex-none;
}

.update-icon {
  @apply w-14 h-14 flex-none;
  filter: drop-shadow(0 8px 14px rgba(32, 184, 137, 0.35));
  -webkit-user-drag: none;
  user-drag: none;
}

.update-heading {
  @apply flex flex-col gap-[3px] pt-[3px] min-w-0;
}

.update-title {
  @apply text-[18px] font-bold tracking-[-0.01em];
}

.update-subtitle {
  @apply text-[12.5px] text-text-secondary;
}

.update-whats-new {
  @apply flex flex-col gap-[7px] flex-1 min-h-0;
}

.update-label {
  @apply text-[11.5px] font-semibold uppercase tracking-[0.06em] text-text-tertiary flex-none;
}

/* The list scrolls inside its own box rather than growing the window: the release notes are as long
   as somebody wrote them, and the window is a fixed size. */
.update-notes {
  @apply flex-1 min-h-[120px] overflow-y-auto rounded-[9px] border border-border bg-bg-inset px-3.5 py-3;
}

.update-note {
  @apply flex gap-[9px] py-1.5 text-[13px] leading-[1.5];
}

.update-note-dot {
  @apply w-1 h-1 rounded-full flex-none mt-[7px];
  background: var(--text-tertiary);
}

.update-empty {
  @apply m-0 text-[13px] text-text-tertiary;
}

.update-actions {
  @apply flex items-center justify-between gap-2.5 pt-1.5 border-t border-border flex-none;
}

.update-skip {
  @apply bg-transparent border-none p-0 mt-1.5 text-[12.5px] text-text-tertiary cursor-pointer;
  font: inherit;
  font-size: 12.5px;
}

.update-skip:hover:not(:disabled) {
  @apply text-text;
  text-decoration: underline;
}

.update-skip:disabled {
  @apply opacity-50 cursor-default;
}

.update-actions-right {
  @apply flex items-center gap-2 mt-1.5;
}

.update-error {
  @apply m-0 text-[12.5px] text-red flex-none;
}
</style>
