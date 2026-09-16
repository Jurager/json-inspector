<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { useSettings } from '../composables/useSettings'
import { useTheme } from '../composables/useTheme'
import { useLocale } from '../composables/useLocale'
import { usePlatform } from '../composables/usePlatform'
import { useUpdateCheck } from '../composables/useUpdateCheck'
import { formatCheckedAt, useMessages } from '../i18n'
import Icon from '../components/ui/Icon.vue'
import UpdateCheck from '../components/update/UpdateCheck.vue'
import { Switch } from '../components/ui/switch'
import { Language, Retention, Theme, UpdateChannel } from '../../bindings/json-inspector/internal/domain'

// The handoff's categories, without the ones the app has nothing to put in yet: proxy, sync and the
// account screen are not built, and a category that opens onto an empty pane is worse than a
// category that is not there.
const CATEGORIES = [
  { id: 'general', icon: 'settings-2', label: 'settings.general' },
  { id: 'appearance', icon: 'contrast', label: 'settings.appearance' },
  { id: 'language', icon: 'globe', label: 'settings.language' },
  { id: 'updates', icon: 'download', label: 'settings.updates' },
] as const

type CategoryId = (typeof CATEGORIES)[number]['id']

const { t } = useMessages()
const { customTitlebar } = usePlatform()
const { theme, setTheme } = useTheme()
const { language, setLanguage } = useLocale()
const { settings, loadSettings, setRetention, setUpdateCheck, setUpdateChannel } = useSettings()
const { checkedAt } = useUpdateCheck()

const active = ref<CategoryId>('general')

// The handoff's order: light, dark, system. The words are the title bar's own — one control, one set
// of names, whichever window it is drawn in.
const THEMES = [
  { value: Theme.ThemeLight, label: 'theme.light' },
  { value: Theme.ThemeDark, label: 'theme.dark' },
  { value: Theme.ThemeSystem, label: 'theme.system' },
] as const

const LANGUAGES = [
  { value: Language.LanguageSystem, label: 'settings.systemLanguage' },
  { value: Language.LanguageRU, label: 'settings.russian' },
  { value: Language.LanguageEN, label: 'settings.english' },
] as const

const RETENTIONS = [
  { value: Retention.RetainWeek, label: 'settings.retentionWeek' },
  { value: Retention.RetainMonth, label: 'settings.retentionMonth' },
  { value: Retention.RetainForever, label: 'settings.retentionForever' },
] as const

const CHANNELS = [
  { value: UpdateChannel.ChannelStable, label: 'settings.channelStable' },
  { value: UpdateChannel.ChannelBeta, label: 'settings.channelBeta' },
] as const

// The stored choice until the window has read it: the same default Go answers with, so the select
// shows something true rather than nothing.
const retention = computed(() => settings.value?.historyRetention ?? Retention.RetainForever)
const autoCheck = computed(() => settings.value?.updateCheckAuto ?? true)
const channel = computed(() => settings.value?.updateChannel ?? UpdateChannel.ChannelStable)

// The date of the last check, which the handoff puts under the switch rather than beside the button:
// it describes how the app behaves on its own, and that is what the switch decides.
const lastChecked = computed(() =>
  checkedAt.value ? t('update.lastChecked', { at: formatCheckedAt(checkedAt.value) }) : ''
)

onMounted(() => {
  void loadSettings()
  // The window's own name is the one thing the catalogue has to give the OS: the title bar it draws
  // is ours, but the taskbar reads the platform's. Go names the window by identity; the words are
  // here, where the language is known.
  void Window.SetTitle(t('settings.title'))
})
</script>

<template>
  <div class="settings-window" :class="{ 'settings-window-mac': !customTitlebar }">
    <header v-if="customTitlebar" class="settings-bar">
      <span class="settings-bar-title">{{ t('settings.title') }}</span>
      <button class="cap-btn cap-close" :title="t('common.close')" @click="Window.Close()">
        <span class="cap-icon cap-icon-close">
          <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
          <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
        </span>
      </button>
    </header>

    <div class="settings-body">
      <nav class="settings-nav">
        <button
          v-for="category in CATEGORIES"
          :key="category.id"
          class="nav-item"
          :class="{ active: active === category.id }"
          @click="active = category.id"
        >
          <Icon :name="category.icon" :size="16" :stroke-width="1.6" />
          <span>{{ t(category.label) }}</span>
        </button>
      </nav>

      <section class="settings-pane">
        <div class="pane-inner">
          <template v-if="active === 'general'">
            <h2 class="pane-title">{{ t('settings.general') }}</h2>
            <div class="row">
              <div class="row-text">
                <div class="row-label">{{ t('settings.retention') }}</div>
              </div>
              <select
                class="select"
                :value="retention"
                @change="setRetention(($event.target as HTMLSelectElement).value as Retention)"
              >
                <option v-for="option in RETENTIONS" :key="option.value" :value="option.value">
                  {{ t(option.label) }}
                </option>
              </select>
            </div>
          </template>

          <template v-else-if="active === 'appearance'">
            <h2 class="pane-title">{{ t('settings.appearance') }}</h2>
            <div class="row">
              <div class="row-text">
                <div class="row-label">{{ t('settings.theme') }}</div>
              </div>
              <div class="segment">
                <button
                  v-for="option in THEMES"
                  :key="option.value"
                  class="segment-item"
                  :class="{ active: theme === option.value }"
                  @click="setTheme(option.value)"
                >
                  {{ t(option.label) }}
                </button>
              </div>
            </div>
          </template>

          <template v-else-if="active === 'language'">
            <h2 class="pane-title">{{ t('settings.language') }}</h2>
            <div class="row">
              <div class="row-text">
                <div class="row-label">{{ t('settings.interfaceLanguage') }}</div>
                <div class="row-hint">{{ t('settings.interfaceLanguageHint') }}</div>
              </div>
              <select
                class="select"
                :value="language"
                @change="setLanguage(($event.target as HTMLSelectElement).value as Language)"
              >
                <option v-for="option in LANGUAGES" :key="option.value" :value="option.value">
                  {{ t(option.label) }}
                </option>
              </select>
            </div>
          </template>

          <template v-else>
            <h2 class="pane-title">{{ t('settings.updates') }}</h2>
            <div class="row">
              <div class="row-text">
                <div class="row-label">{{ t('settings.checkAutomatically') }}</div>
                <div v-if="lastChecked" class="row-hint">{{ lastChecked }}</div>
              </div>
              <Switch
                :model-value="autoCheck"
                :aria-label="t('settings.checkAutomatically')"
                @update:model-value="setUpdateCheck(!!$event)"
              />
            </div>
            <div class="row">
              <div class="row-text">
                <div class="row-label">{{ t('settings.channel') }}</div>
              </div>
              <select
                class="select"
                :value="channel"
                @change="
                  setUpdateChannel(($event.target as HTMLSelectElement).value as UpdateChannel)
                "
              >
                <option v-for="option in CHANNELS" :key="option.value" :value="option.value">
                  {{ t(option.label) }}
                </option>
              </select>
            </div>
            <div class="row row-plain">
              <UpdateCheck inline />
            </div>
          </template>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

/* An opaque window: it shows no material behind it, so the page paints the whole ground. */
.settings-window {
  @apply h-full flex flex-col bg-bg-panel text-text select-none;
}

/* macOS hides its title bar inside the window, so the content starts below it; the number matches
   `InvisibleTitleBarHeight` in host.go. */
.settings-window-mac .settings-body {
  padding-top: 50px;
}

.settings-bar {
  @apply relative flex-none h-13 flex items-center justify-center;
  border-bottom: 1px solid var(--glass-chrome-border);
  background: var(--glass-chrome);
  backdrop-filter: var(--blur-chrome);
  --wails-draggable: drag;
}

.settings-bar-title {
  @apply text-[13px] font-semibold text-text;
}

.settings-bar .cap-btn {
  @apply absolute right-0;
}

.settings-body {
  @apply flex-1 flex min-h-0;
}

.settings-nav {
  @apply flex-none w-[208px] flex flex-col gap-0.5 p-[14px_10px];
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
}

.nav-item {
  @apply flex items-center gap-[9px] w-full py-2 px-2.5 border-none rounded-[7px] bg-transparent text-text text-[12.5px] text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.nav-item:hover {
  @apply text-text;
  background: var(--bg-hover);
}

.nav-item.active {
  @apply bg-accent-soft text-accent font-semibold;
}

.nav-item:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.settings-pane {
  @apply flex-1 min-w-0 overflow-auto p-[28px_36px];
}

/* The handoff's measure: wide enough for a sentence of description beside its control, and no wider,
   so a row stays one line of reading rather than a page. */
.pane-inner {
  @apply flex flex-col gap-0.5 max-w-[560px];
}

.pane-title {
  @apply m-0 mb-2.5 text-[20px] font-semibold tracking-[-0.01em];
}

.row {
  @apply flex items-start justify-between gap-6 py-3.5;
  border-bottom: 1px solid var(--border);
}

/* The check button and its answer are one row of controls, not a label with a control beside it:
   they take the whole width and start at the left edge. */
.row-plain {
  @apply justify-start;
}

.row-text {
  @apply flex flex-col gap-[3px];
}

.row-label {
  @apply text-[13px] font-semibold;
}

.row-hint {
  @apply text-xs leading-[1.5] text-text-secondary;
}

.select {
  @apply flex-none h-[30px] px-2.5 rounded-[7px] text-[12.5px] text-text cursor-pointer;
  font: inherit;
  font-size: 12.5px;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
}

/* One track with the chosen segment raised in it, exactly as the handoff draws it — a text segment,
   not the title bar's icon one: the same three choices, read as words. */
.segment {
  @apply flex-none flex gap-0.5 p-0.5 rounded-lg;
  width: 260px;
  background: var(--bg-inset);
}

.segment-item {
  @apply flex-1 text-center py-1.5 px-2 rounded-md border-none bg-transparent text-text-secondary text-xs cursor-pointer;
  font: inherit;
  font-size: 12px;
  --wails-draggable: no-drag;
}

.segment-item.active {
  @apply bg-bg-panel text-text font-semibold;
  box-shadow: var(--shadow);
}
</style>
