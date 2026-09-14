<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Window } from '@wailsio/runtime'
import { SystemService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import EnvironmentMenu from '../environments/EnvironmentMenu.vue'
import ThemeSwitch from './ThemeSwitch.vue'
import WorkspaceSwitcher from '../workspaces/WorkspaceSwitcher.vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import logoUrl from '../../assets/logo.svg'
import { DropdownMenu, DropdownMenuTrigger } from '../ui/dropdown-menu'

const { t } = useMessages()

const store = useRequestsStore()
const envStore = useEnvironmentsStore()
const { customTitlebar, shortcut } = usePlatform()

const appName = ref('')
const isMaximised = ref(false)

const searchHint = computed(() => shortcut('K'))

const activeEnvName = computed(() => envStore.activeEnvironment?.name ?? t('titlebar.noEnvironment'))

const ENV_DOT_COLORS: Record<string, string> = {
  green: 'var(--green)',
  orange: 'var(--orange)',
  red: 'var(--red)',
  purple: 'var(--purple)',
}

const envDotStyle = computed(() => {
  const env = envStore.activeEnvironment
  if (!env) return { background: 'var(--text-tertiary)' }
  return { background: ENV_DOT_COLORS[env.color ?? 'green'] ?? 'var(--green)' }
})

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

    <div v-if="customTitlebar" class="titlebar-spacer"></div>

    <div class="titlebar-actions" :class="{ 'titlebar-actions-flush': customTitlebar }">
      <div class="env-wrap">
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button :title="t('titlebar.environment', { name: activeEnvName })">
              <span class="env-dot" :style="envDotStyle"></span>
              <span>{{ activeEnvName }}</span>
              <Icon name="chevron-down" :size="11" />
            </Button>
          </DropdownMenuTrigger>
          <EnvironmentMenu />
        </DropdownMenu>
      </div>
      <ThemeSwitch />
      <Button
        class="titlebar-search"
        :title="t('titlebar.searchHint', { shortcut: searchHint })"
        @click="store.focusSearch()"
      >
        <span>{{ t('common.search') }}</span>
        <kbd class="titlebar-key">{{ searchHint }}</kbd>
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

.btn.titlebar-search {
  color: var(--text-secondary);
}

.titlebar-key {
  @apply text-xs leading-none px-1 py-0.5 rounded-sm text-text-tertiary;
  font-family: var(--mono);
  background: var(--bg-inset);
  border: 1px solid var(--border);
}
</style>
