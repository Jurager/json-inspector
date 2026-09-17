<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Window } from '@wailsio/runtime'
import { useSettings } from '../composables/useSettings'
import { usePlatform } from '../composables/usePlatform'
import { useSheetNotice } from '../composables/useSheetNotice'
import { useMessages } from '../i18n'
import SettingsRail from '../components/settings/SettingsRail.vue'
import GeneralPane from '../components/settings/GeneralPane.vue'
import AppearancePane from '../components/settings/AppearancePane.vue'
import RequestsPane from '../components/settings/RequestsPane.vue'
import UpdatesPane from '../components/settings/UpdatesPane.vue'
import StubPane from '../components/settings/StubPane.vue'
import { CATEGORIES, type CategoryId } from '../components/settings/categories'

// The window is the rail plus one category: the drawing's frame, drawn inside a window of its own. The
// backdrop and the "Done" button the drawing puts around it belong to an overlay raised over another
// window, and this one has a title bar of its own — the platform's on macOS, ours on Windows — which
// is what closes it.
//
// The four categories the app has nothing for are drawn all the same: their rows are the design's,
// every control is off, and each says in its first card why.
const { t } = useMessages()
const { customTitlebar } = usePlatform()
const { loadSettings } = useSettings()
const { notice, clearNotice } = useSheetNotice()

const active = ref<CategoryId>('general')

const isLive = computed(() =>
  CATEGORIES.some((category) => category.id === active.value && !category.soon)
)

function select(id: CategoryId) {
  active.value = id
  // A failure belongs to the row that caused it, and the row is gone with the category.
  clearNotice()
}

onMounted(() => {
  void loadSettings()
  // The window's own name is the one thing the catalogue has to give the OS: the title bar it draws is
  // ours, but the taskbar reads the platform's. Go names the window by identity; the words are here,
  // where the language is known.
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
      <SettingsRail :active="active" @select="select" />

      <section class="settings-pane">
        <div class="pane-scroll">
          <GeneralPane v-if="active === 'general'" />
          <AppearancePane v-else-if="active === 'appearance'" />
          <RequestsPane v-else-if="active === 'requests'" />
          <UpdatesPane v-else-if="active === 'updates'" />
          <StubPane v-else-if="!isLive" :category="active" />
        </div>

        <!-- A write that failed says so under the rows it was about, and stays there until the row is
             tried again or another category is opened. -->
        <div v-if="notice" class="pane-notice">{{ notice }}</div>
      </section>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

/* An opaque window: it shows no material behind it, so the page paints the whole ground. The body is
   the window's own base colour rather than a panel's, because the rows' controls are panels: a field
   the same white as the page behind it is a field nobody can see.
   The drawing is plain HTML and draws every line at `normal`, while the window's base is Tailwind's
   1.5 — four pixels a row, which is a section taller than it is drawn. */
.settings-window {
  @apply h-full flex flex-col bg-bg text-text select-none;
  line-height: normal;
}

/* macOS hides its title bar inside the window, so the content starts below it; the number matches
   `InvisibleTitleBarHeight` in host.go. */
.settings-window-mac .settings-body {
  padding-top: 50px;
}

/* Left, like the main window's titlebar and like the platform: the design centres it because it draws
   macOS, where the system draws the title and this bar does not exist. */
.settings-bar {
  @apply relative flex-none h-13 flex items-center justify-start px-3;
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

.settings-pane {
  @apply flex-1 min-w-0 flex flex-col;
}

/* The drawing's own measure around the sections: 18 down, 20 across. */
.pane-scroll {
  @apply flex-1 min-h-0 overflow-y-auto;
  padding: 18px 20px;
}

.pane-notice {
  @apply flex-none text-[12.5px];
  padding: 10px 20px;
  border-top: 1px solid var(--border);
  color: var(--red-text);
}
</style>
