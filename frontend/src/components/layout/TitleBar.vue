<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Window } from '@wailsio/runtime'
import { SystemService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useSearchStore } from '../../stores/search'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import EnvironmentButton from '../environments/EnvironmentButton.vue'
import ThemeSwitch from './ThemeSwitch.vue'
import WorkspaceSwitcher from '../workspaces/WorkspaceSwitcher.vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import logoUrl from '../../assets/logo.svg'

const { t } = useMessages()

const search = useSearchStore()
const { customTitlebar, shortcut } = usePlatform()

const appName = ref('')
const isMaximised = ref(false)

const searchHint = computed(() => shortcut('K'))

async function loadAppName() {
  try {
    appName.value = (await SystemService.Name()) ?? ''
  } catch {
    // Nothing else depends on the name; the title bar just stays empty.
  }
}

async function refreshMaximised() {
  try {
    isMaximised.value = await Window.IsMaximised()
  } catch {
    // Runtime not ready yet.
  }
}

watch(
  customTitlebar,
  (custom) => {
    if (custom) {
      refreshMaximised()
      window.addEventListener('resize', refreshMaximised)
    }
  },
  { immediate: true }
)

onMounted(() => {
  loadAppName()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', refreshMaximised)
})
</script>

<template>
  <header class="titlebar" :class="{ 'titlebar-custom': customTitlebar }" @dblclick="SystemService.ToggleMaximize">
    <div v-if="customTitlebar" class="titlebar-appicon">
      <img :src="logoUrl" alt="" class="titlebar-logo" draggable="false" />
      <span class="titlebar-title">{{ appName }}</span>
    </div>
    <span v-else class="titlebar-title">{{ appName }}</span>

    <!-- Which workspace the window is showing, in the corner the mockup puts it: on macOS it stands
         where the traffic lights end, and on a titlebar we draw ourselves it follows the app mark. -->
    <div class="titlebar-workspace" :class="{ 'titlebar-workspace-mac': !customTitlebar }">
      <WorkspaceSwitcher />
    </div>

    <!-- Where the bar is ours, the environment stands in the middle of it and the controls sit
         beside it on the right: which space this is, which variables it sends with, in one line.
         On macOS the middle is the system's title, so the same button keeps its place among the
         controls instead. -->
    <template v-if="customTitlebar">
      <div class="titlebar-spacer"></div>
      <EnvironmentButton />
      <div class="titlebar-spacer"></div>
    </template>

    <div class="titlebar-actions" :class="{ 'titlebar-actions-flush': customTitlebar }">
      <EnvironmentButton v-if="!customTitlebar" />
      <ThemeSwitch />
      <!-- A magnifier and the key, no word: the bar carries the shortcut the way the rest of the
           window does, and the tooltip has the sentence for anyone who needs it. -->
      <Button
        class="titlebar-search"
        :title="t('titlebar.searchHint', { shortcut: searchHint })"
        :aria-label="t('common.search')"
        @click="search.openPalette()"
      >
        <Icon name="search" :size="15" :stroke-width="1.9" />
        <span class="titlebar-key">{{ searchHint }}</span>
      </Button>
    </div>

    <div v-if="customTitlebar" class="titlebar-controls">
      <button class="cap-btn" :title="t('titlebar.minimise')" @click="Window.Minimise()">
        <span class="cap-icon cap-icon-minus"></span>
      </button>
      <button class="cap-btn" :title="t('titlebar.maximise')" @click="Window.ToggleMaximise()">
        <span v-if="!isMaximised" class="cap-icon cap-icon-square"></span>
        <span v-else class="cap-icon cap-icon-restore">
          <span class="cap-icon cap-icon-restore-back"></span>
          <span class="cap-icon cap-icon-restore-front"></span>
        </span>
      </button>
      <button class="cap-btn cap-close" :title="t('common.close')" @click="Application.Quit()">
        <span class="cap-icon cap-icon-close">
          <span class="cap-icon-close-bar cap-icon-close-bar-1"></span>
          <span class="cap-icon-close-bar cap-icon-close-bar-2"></span>
        </span>
      </button>
    </div>
  </header>
</template>

<style scoped>
@reference "../../style.css";

/* The switcher keeps its own corner. On macOS the title is centred and the traffic lights are drawn
   over the bar by the system, so the pill is taken out of the row and pinned past them; on a
   titlebar of our own the row is laid out from the left and the pill simply follows the app mark. */
.titlebar-workspace {
  @apply flex items-center;
  --wails-draggable: no-drag;
}

/* macOS draws its own controls over this bar and the latest versions lay them out wider than they
   used to: the last of the three ends 78px from the window's edge, so the pill starts 12px past it —
   the same gap the mockup leaves between the lights and the switcher. */
.titlebar-workspace-mac {
  @apply absolute top-1/2 -translate-y-1/2;
  left: 90px;
}

/* The search is a chip and not a button: no frame and no shadow, the same fill the chips in the
   command line carry, and the shortcut written beside it in the tertiary ink. */
.btn.titlebar-search {
  @apply text-text-secondary;
  height: 28px;
  padding: 0 9px;
  gap: 7px;
  border: 0;
  background: var(--bg-hover);
  box-shadow: none;
}

.btn.titlebar-search:hover:not(:disabled) {
  background: var(--bg-active);
  color: var(--text);
}

/* The shortcut is written in the bar's own face, not in the mono one: the handoff draws every key
   inside a button this way, and the mono face is kept for values. */
.titlebar-key {
  @apply text-[12px] leading-none text-text-tertiary;
}
</style>
