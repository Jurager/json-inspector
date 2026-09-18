<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Events, Window } from '@wailsio/runtime'
import { useSettings } from '../composables/useSettings'
import { useAccount } from '../composables/useAccount'
import SignInModal from '../components/settings/SignInModal.vue'
import SignOutSheet from '../components/settings/SignOutSheet.vue'
import { usePlatform } from '../composables/usePlatform'
import { useSheetNotice } from '../composables/useSheetNotice'
import { useMessages } from '../i18n'
import SettingsRail from '../components/settings/SettingsRail.vue'
import AccountPane from '../components/settings/AccountPane.vue'
import GeneralPane from '../components/settings/GeneralPane.vue'
import AppearancePane from '../components/settings/AppearancePane.vue'
import RequestsPane from '../components/settings/RequestsPane.vue'
import UpdatesPane from '../components/settings/UpdatesPane.vue'
import StubPane from '../components/settings/StubPane.vue'
import { asCategory, CATEGORIES, type CategoryId } from '../components/settings/categories'

// The window is the rail plus one category: the drawing's frame, drawn inside a window of its own. The
// backdrop and the "Done" button the drawing puts around it belong to an overlay raised over another
// window, and this one has a title bar of its own — the platform's on macOS, ours on Windows — which
// is what closes it.
//
// The three categories the app has nothing for are drawn all the same: their rows are the design's,
// every control is off, and each says in its first card why.
const { t } = useMessages()
const { customTitlebar } = usePlatform()
const { loadSettings } = useSettings()
const { notice, clearNotice } = useSheetNotice()
const {
  state: accountState,
  signInOpen,
  signOutOpen,
  beginSignIn,
  cancelSignIn,
  closeSignIn,
  closeSignOut,
  signOut,
} = useAccount()
const account = computed(() => accountState.value?.account ?? null)

// The category is opened on the one asked for: Go puts it on the address of a window it is about to
// create, and tells a window that is already on screen — a page reads no URL twice.
const active = ref<CategoryId>(asCategory(new URLSearchParams(location.search).get('tab')))

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

  // A window that is already open is asked to switch by an event: the account menu sends people to
  // the account, and a page that has been painted reads no address again.
  Events.On('settings-tab', (event) => {
    const asked = event.data as string
    if (asked) select(asCategory(asked))
  })
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
          <AccountPane v-if="active === 'account'" />
          <GeneralPane v-else-if="active === 'general'" />
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

    <!-- The account's two dialogs, raised from the rows on this page and from the rail's menu in the
         other window. Each window draws its own: the flag travels with the window, not the account. -->
    <SignInModal
      v-if="signInOpen"
      :server="account?.server ?? ''"
      @begin="beginSignIn"
      @cancel="cancelSignIn"
      @close="closeSignIn()"
    />
    <SignOutSheet v-if="signOutOpen" @close="closeSignOut()" @confirm="signOut()" />
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
  @apply flex-none text-[13px];
  padding: 10px 20px;
  border-top: 1px solid var(--border);
  color: var(--red-text);
}
</style>
