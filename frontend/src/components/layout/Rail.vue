<script setup lang="ts">
import {
  RecordsService,
  SystemService,
  UpdateService,
} from '../../../bindings/json-inspector/internal/transport/wails'
import { computed, ref } from 'vue'
import { useEnvironmentsStore } from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { buildSampleRecord } from '../../lib/sample'
import { useMessages } from '../../i18n'
import { usePlatform } from '../../composables/usePlatform'
import { useAccount } from '../../composables/useAccount'
import Icon from '../ui/Icon.vue'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'
import { Popover, PopoverTrigger, PopoverContent, PopoverClose } from '../ui/popover'

const { t } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()
const { customTitlebar } = usePlatform()
const { state, loadAccount, openSignIn, openSignOut } = useAccount()

// The account button: the drawing's chair at the bottom of the rail, with a dot while somebody is
// signed in. What the menu then offers depends on which of the two states it is in.
const accountOpen = ref(false)
const account = computed(() => state.value?.account ?? null)
const signedIn = computed(() => state.value?.signedIn === true)
const accountTitle = computed(() =>
  account.value?.email ? `${t('rail.account')} · ${account.value.email}` : t('rail.account')
)

// Opening the menu reads the account rather than trusting what the last event left behind: a window
// that has been in the background since a token was refused would offer a sign-out that is not due.
function onAccountOpen(open: boolean) {
  if (open) void loadAccount()
}

// A tile holds a message key rather than its label: which tile is which does not change with the
// language, and looking the words up as it is drawn is what lets them change without a reload.
const RAIL_ITEMS = [
  { view: 'request', icon: 'arrow-up-right', label: 'rail.request' },
  { view: 'browser', icon: 'record', label: 'rail.browser' },
  { view: 'collections', icon: 'folder', label: 'rail.collections' },
] as const

type RailView = (typeof RAIL_ITEMS)[number]['view']

async function loadSample() {
  const record = await RecordsService.Ingest(buildSampleRecord())
  // The event that announces it may not have reached the window yet, and the pane cannot select a
  // record the mirror does not hold.
  store.prepend(record)
  store.activeView = 'request'
  // A sample is meant to be played with, so it opens in the request as well as in the pane.
  await store.selectManual(record.id)
}

function openAbout() {
  SystemService.ShowAbout()
}

// The gear is the settings window, as the handoff has it. The environments sheet keeps its two other
// doors — the title bar's dropdown and its own shortcut — so nothing lost a way in.
//
// The category is only named by the account menu: the gear opens the window where it was left, and
// an empty category is what says so.
function openSettings(category = '') {
  SystemService.ShowSettings(category)
}

// The rail's update badge asks the About window to check, which is where the answer is drawn: it is
// a window of its own, and this one cannot show the result.
function requestUpdateCheck() {
  UpdateService.RequestCheck()
}

async function selectSource(view: RailView) {
  // A card with unsaved edits is not left quietly, whichever way the user leaves it.
  if (view !== 'collections' && !(await collections.askUnsaved())) return
  if (view === 'browser') store.clearUnreadCaptures()
  store.activeView = view
}
</script>

<template>
  <aside class="sidebar">
    <!-- The hamburger opens the rail, the gear closes it. Neither sits in a strip with a hairline any
         more: the rail is one column of buttons now, and the lines it used to carry across the window
         are gone with them. -->
    <div class="rail-head">
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <button class="rail-btn" :title="t('rail.menu')">
            <Icon name="menu" :size="21" :stroke-width="1.8" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          <DropdownMenuItem @select="loadSample">
            <Icon name="sparkles" :size="14" /> {{ t('rail.loadSample') }}
          </DropdownMenuItem>
          <DropdownMenuItem @select="requestUpdateCheck">
            <Icon name="arrow-down" :size="14" /> {{ t('rail.checkUpdates') }}
          </DropdownMenuItem>
          <!-- On macOS this lives in the native app menu instead. -->
          <DropdownMenuItem v-if="customTitlebar" @select="openAbout">
            <Icon name="info" :size="14" /> {{ t('rail.about') }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <div class="rail-body">
      <button
        v-for="item in RAIL_ITEMS"
        :key="item.view"
        class="rail-item"
        :class="{ active: store.activeView === item.view }"
        @click="selectSource(item.view)"
      >
        <span class="rail-icon"><Icon :name="item.icon" :size="23" :stroke-width="1.7" /></span>
        <span class="rail-label">{{ t(item.label) }}</span>
        <span v-if="item.view === 'browser' && store.unreadCount > 0" class="rail-badge">
          {{ store.unreadCount }}
        </span>
      </button>

      <div class="rail-spacer"></div>
    </div>

    <!-- Settings has a footer of its own: it is not the last thing in the rail's list of sources but
         the door out of it. The account sits above it — the drawing's chair — and its menu is the
         one place in the main window where a person signs in or out without opening the settings. -->
    <div class="rail-footer">
      <Popover v-model:open="accountOpen" @update:open="onAccountOpen">
        <PopoverTrigger as-child>
          <button class="rail-btn relative" :title="accountTitle">
            <Icon name="user" :size="20" :stroke-width="1.7" />
            <!-- The dot is presence, not a badge: it says the app holds an account, and it is drawn
                 only then. -->
            <span v-if="signedIn" class="rail-presence"></span>
          </button>
        </PopoverTrigger>
        <PopoverContent class="account-menu" side="right" align="end">
          <div class="account-head">
            <span class="account-face"><Icon name="user" :size="18" :stroke-width="1.7" /></span>
            <span class="account-who">
              <span class="account-name">{{ account?.email || t('account.signedOut') }}</span>
              <span class="account-meta">{{ account?.server ?? '' }}</span>
            </span>
          </div>

          <PopoverClose as-child>
            <button class="account-row" @click="openSettings('account')">
              <Icon name="user" :size="15" :stroke-width="1.7" />
              {{ t('settings.sec.account') }}
            </button>
          </PopoverClose>
          <PopoverClose as-child>
            <button class="account-row" @click="openSettings()">
              <Icon name="settings-2" :size="15" :stroke-width="1.8" />
              {{ t('settings.title') }}
            </button>
          </PopoverClose>

          <div class="account-divider"></div>

          <PopoverClose v-if="signedIn" as-child>
            <button class="account-row bad" @click="openSignOut()">
              <Icon name="logout" :size="15" :stroke-width="1.8" />
              {{ t('account.signOut') }}
            </button>
          </PopoverClose>
          <PopoverClose v-else as-child>
            <button class="account-row good" @click="openSignIn()">
              <Icon name="user" :size="15" :stroke-width="1.7" />
              {{ t('account.signIn') }}
            </button>
          </PopoverClose>
        </PopoverContent>
      </Popover>

      <button class="rail-btn" :title="t('rail.settings')" @click="openSettings()">
        <Icon name="settings-2" :size="21" :stroke-width="1.8" />
      </button>
    </div>
  </aside>
</template>
