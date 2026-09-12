<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Application, Window } from '@wailsio/runtime'
import { App as Backend } from '../../../bindings/json-inspector'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { shortcut, useCustomTitlebar } from '../../lib/platform'
import EnvironmentMenu from '../environments/EnvironmentMenu.vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import logoUrl from '../../assets/logo.svg'
import { DropdownMenu, DropdownMenuTrigger } from '../ui/dropdown-menu'

const store = useRequestsStore()
const envStore = useEnvironmentsStore()

const appName = ref('')
const isMaximised = ref(false)

// Reactive: the platform resolves shortly after load (see lib/platform.ts).
const searchHint = computed(() => shortcut('K'))

// Environment is a property of the window, not of a request: the chip names the active one and is
// the only way in — there is no rail section, by design.
const activeEnvName = computed(() => envStore.active?.name ?? 'Без окружения')

const ENV_DOT_COLORS: Record<string, string> = {
  green: 'var(--green)',
  orange: 'var(--orange)',
  red: 'var(--red)',
  purple: 'var(--purple)',
}

const envDotStyle = computed(() => {
  const env = envStore.active
  // No environment: the dot goes neutral rather than claiming a state.
  if (!env) return { background: 'var(--text-tertiary)' }
  return { background: ENV_DOT_COLORS[env.color ?? 'green'] ?? 'var(--green)' }
})

// The name comes from the backend, so it can't drift from the window title and the macOS menu bar;
// it arrives a beat after the first paint.
async function loadAppName() {
  try {
    appName.value = (await Backend.Name()) ?? ''
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

// Maximise state is only shown by our own caption buttons, which exist on the frameless platforms
// only — and the platform resolves asynchronously, so watch it rather than check once.
watch(
  useCustomTitlebar,
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
  <header class="titlebar" :class="{ 'titlebar-custom': useCustomTitlebar }" @dblclick="Backend.ToggleMaximize">
    <div v-if="useCustomTitlebar" class="titlebar-appicon">
      <img :src="logoUrl" alt="" class="titlebar-logo" draggable="false" />
      <span class="titlebar-title">{{ appName }}</span>
    </div>
    <span v-else class="titlebar-title">{{ appName }}</span>

    <div v-if="useCustomTitlebar" class="titlebar-spacer"></div>

    <!-- The chip and search belong to both titlebars: on macOS they float over the native
         hidden-inset bar, on the frameless one they sit in the flex row before the caption buttons.
    -->
    <div class="titlebar-actions" :class="{ 'titlebar-actions-flush': useCustomTitlebar }">
      <div class="env-wrap">
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button :title="`Окружение: ${activeEnvName}`">
              <span class="env-dot" :style="envDotStyle"></span>
              <span>{{ activeEnvName }}</span>
              <Icon name="chevron-down" :size="11" />
            </Button>
          </DropdownMenuTrigger>
          <EnvironmentMenu />
        </DropdownMenu>
      </div>
      <Button class="titlebar-search" :title="`Глобальный поиск (${searchHint})`" @click="store.focusSearch()">
        <span>Поиск</span>
        <kbd class="titlebar-key">{{ searchHint }}</kbd>
      </Button>
    </div>

    <div v-if="useCustomTitlebar" class="titlebar-controls">
      <button class="cap-btn" title="Свернуть" @click="Window.Minimise()">
        <span class="cap-icon cap-icon-minus"></span>
      </button>
      <button class="cap-btn" title="Развернуть" @click="Window.ToggleMaximise()">
        <span v-if="!isMaximised" class="cap-icon cap-icon-square"></span>
        <span v-else class="cap-icon cap-icon-restore">
          <span class="cap-icon-restore-back"></span>
          <span class="cap-icon-restore-front"></span>
        </span>
      </button>
      <button class="cap-btn cap-close" title="Закрыть" @click="Application.Quit()">
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

/* Names .btn to outrank the colour the primitive sets on its own root. */
.btn.titlebar-search {
  color: var(--text-secondary);
}

.titlebar-key {
  @apply text-[10.5px] leading-none px-1 py-0.5 rounded-sm text-text-tertiary;
  font-family: var(--mono);
  background: var(--bg-inset);
  border: 1px solid var(--border);
}
</style>
