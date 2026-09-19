<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import ValueChip from '../ui/ValueChip.vue'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import { Sheet } from '../ui/sheet'
import type { Session } from '../../../bindings/json-inspector/internal/domain'
import { useAccount } from '../../composables/useAccount'
import { formatDate, useMessages } from '../../i18n'

// The account. Two shapes, and the app is usable in both: nobody signed in — collections, environments
// and history live here and only here — and a person signed in, whose other devices are listed and can
// be ended one by one.
//
// The row about sync is drawn as the stubs are: switched off, with one sentence saying why. Sync does
// not exist yet, and a switch that looked operable would be a promise the app cannot keep.
const { t } = useMessages()
const {
  state,
  serverDown,
  lastReach,
  sessions,
  failure,
  sessionsFailed,
  busy,
  signInOpen,
  signOutOpen,
  openSignIn,
  openSignOut,
  loadAccount,
  loadSessions,
  endSession,
  endOthers,
  deleteAccount,
  checkServer,
} = useAccount()

// Only the deletion is the pane's own sheet: signing in and signing out are the account's two
// dialogs, raised by the window root so that the rail's menu can raise the same ones.
const leaving = ref(false)

const account = computed(() => state.value?.account ?? null)
const signedIn = computed(() => state.value?.signedIn === true)
const thisSession = computed(() => account.value?.sessionId ?? '')
const server = computed(() => account.value?.server ?? '')

// The server's own row in the block about the wire: which server it is, and when it last answered.
// The address is a choice kept on this machine, so there is one to name even while it is silent.
const serverNote = computed(() =>
  [server.value, lastReach.value ? t('account.lastReach', { when: lastReach.value }) : '']
    .filter(Boolean)
    .join(' · ')
)

// What the account's own row says under the email: the row is kept here, so while the server does not
// answer the details beside it are the ones from its last reply rather than the current ones.
const profileNote = computed(() =>
  serverDown.value
    ? t('account.cachedProfile', { when: lastReach.value || t('account.never') })
    : t('account.signedInAs')
)

// The tariff block is drawn only when the server says something other than free. A self-hosted server
// answers free, and "free · 1 seat" is a tariff nobody can buy. While the server is away there is
// nothing to draw at all: the block would be the last reply, and the connection block says so.
const plan = computed(() => {
  if (serverDown.value) return null
  const held = account.value
  if (!held || !held.plan || held.plan === 'free') return null
  return { name: held.plan, seats: held.seats }
})

onMounted(async () => {
  // Asked rather than read: this pane is where a person comes to see what the server says about the
  // devices and the plan, and the row on this machine is only half of that. The list of devices is a
  // call of its own, and it goes out only when there is an account to have devices on.
  await checkServer()
  if (signedIn.value) void loadSessions()
})

// The devices are read when the account arrives, and a sign-in is the one moment the pane knows it
// did not have an account a second ago. The dialog belongs to the window, so the pane watches it
// rather than being told by it: both are open at once, and the one that can see the list reloads it.
watch([signInOpen, signOutOpen], ([signing, leaving]) => {
  if (signing || leaving) return
  void loadAccount()
  void loadSessions()
})

/** A device as the list reads it: "Windows · JSON Inspector 2.3.6" — where it runs, and which build. */
function describe(device: Session['device']): string {
  const program = [device.name, device.appVersion].filter(Boolean).join(' ')
  return [device.platform, program].filter(Boolean).join(' · ')
}

/** When a device signed in, in the words the language writes dates with. */
function when(value: string): string {
  const at = new Date(value)
  if (Number.isNaN(at.getTime())) return ''
  return formatDate(at, { day: 'numeric', month: 'long', hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="pane">
    <SettingsSection v-if="!signedIn" :label="t('settings.sec.account')">
      <SettingsRow :title="t('account.signedOut')" :note="t('account.signedOutNote')">
        <Button variant="primary" size="field" :disabled="busy" @click="openSignIn()">
          {{ t('account.signIn') }}
        </Button>
      </SettingsRow>
      <!-- A server of one's own is a self-hosted deployment, and this row is about that and nothing
           else: on the server the product ships with there is no address to show, and showing the
           program's own would be showing a setting nobody made. -->
      <SettingsRow v-if="server" :title="t('account.server')" :note="t('account.serverNote')">
        <ValueChip :value="server" mono />
      </SettingsRow>
    </SettingsSection>

    <template v-else>
      <!-- The block the design adds for the state where the account is here and the server is not:
           what still works, what can be done about it, and what the pane cannot show at all. It
           stands above everything else because it is the reason the rest looks the way it does. -->
      <SettingsSection v-if="serverDown" :label="t('account.connection')" :note="t('account.connectionNote')">
        <SettingsRow :title="t('account.serverDown')" :note="serverNote">
          <Button variant="primary" size="field" @click="checkServer()">
            {{ t('account.retry') }}
          </Button>
        </SettingsRow>
        <SettingsRow :title="t('account.details')" :note="t('account.detailsNote')">
          <ValueChip :value="t('account.unavailable')" />
        </SettingsRow>
      </SettingsSection>

      <SettingsSection :label="t('settings.sec.account')">
        <SettingsRow :title="account?.email ?? ''" :note="profileNote">
          <ValueChip v-if="server" :value="server" mono />
        </SettingsRow>
        <SettingsRow :title="t('account.sync')" :note="t('account.syncSoon')">
          <Switch :model-value="false" disabled />
        </SettingsRow>
        <SettingsRow :title="t('account.signOut')" :note="t('account.signOutNote')">
          <Button variant="outline" size="field" @click="openSignOut()">
            {{ t('account.signOut') }}
          </Button>
        </SettingsRow>
      </SettingsSection>

      <!-- The list of devices lives on the server, so with no answer there is no list to draw and
           nothing to end: what is known locally is drawn instead — this machine is still signed in,
           and the rest is not a fact this window has. -->
      <SettingsSection
        :label="t('account.devices')"
        :note="serverDown ? t('account.devicesOffline') : t('account.devicesNote')"
      >
        <template v-if="serverDown">
          <SettingsRow :title="t('account.thisDevice')" :note="t('account.thisDeviceLocal')">
            <ValueChip :value="t('account.active')" />
          </SettingsRow>
          <SettingsRow :title="t('account.otherDevices')" :note="t('account.otherDevicesNote')">
            <ValueChip :value="t('account.unavailable')" />
          </SettingsRow>
        </template>
        <SettingsRow
          v-for="session in serverDown ? [] : sessions"
          :key="session.id"
          :title="describe(session.device) || t('account.devices')"
          :note="[when(session.createdAt), session.ip].filter(Boolean).join(' · ')"
        >
          <ValueChip v-if="session.id === thisSession" :value="t('account.thisDevice')" />
          <!-- Ending a session is a call to the server, and the server is there: this is only drawn
               while it answers. -->
          <Button v-else variant="outline" size="field" @click="endSession(session.id)">
            {{ t('account.end') }}
          </Button>
        </SettingsRow>
        <SettingsRow
          v-if="!serverDown && sessions.length === 0"
          :title="t('account.devicesEmpty')"
          :note="sessionsFailed ? t('account.devicesFailed') : ''"
        />
        <SettingsRow
          v-if="!serverDown && sessions.length > 1"
          :title="t('account.endOthers')"
          :note="t('account.endOthersNote')"
        >
          <Button variant="outline" size="field" @click="endOthers">
            {{ t('account.endOthers') }}
          </Button>
        </SettingsRow>
      </SettingsSection>

      <SettingsSection v-if="plan" :label="t('account.plan')">
        <SettingsRow :title="plan.name" :note="`${t('account.seats')}: ${plan.seats}`" />
      </SettingsSection>

      <SettingsSection :label="t('settings.sec.data')">
        <SettingsRow :title="t('account.leave')" :note="t('account.leaveNote')">
          <!-- Leaving for good is the server's to do and nobody else's: an account deleted on this
               machine alone is an account that is still there. -->
          <Button
            variant="danger"
            size="field"
            :disabled="serverDown"
            :title="serverDown ? t('account.serverDown') : ''"
            @click="leaving = true"
          >
            {{ t('account.leaveAction') }}
          </Button>
        </SettingsRow>
      </SettingsSection>
    </template>

    <p v-if="failure" class="bad">{{ failure }}</p>

    <Sheet
      v-if="leaving"
      :title="t('account.leaveConfirm')"
      :sub="t('account.leaveText')"
      :cancel="t('common.cancel')"
      :action="t('account.leaveAction')"
      @close="leaving = false"
      @cancel="leaving = false"
      @action="
        () => {
          leaving = false
          deleteAccount()
        }
      "
    />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.pane {
  @apply flex flex-col;
  gap: 18px;
}

/* A refusal stands under the rows it was about and is worded by the catalogue — a code never reaches
   the screen. */
.bad {
  @apply m-0 text-[13px] text-red;
}
</style>
