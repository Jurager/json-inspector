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
} = useAccount()

// Only the deletion is the pane's own sheet: signing in and signing out are the account's two
// dialogs, raised by the window root so that the rail's menu can raise the same ones.
const leaving = ref(false)

const account = computed(() => state.value?.account ?? null)
const signedIn = computed(() => state.value?.signedIn === true)
const thisSession = computed(() => account.value?.sessionId ?? '')
const server = computed(() => account.value?.server ?? '')

// The tariff block is drawn only when the server says something other than free. A self-hosted server
// answers free, and "free · 1 seat" is a tariff nobody can buy.
const plan = computed(() => {
  const held = account.value
  if (!held || !held.plan || held.plan === 'free') return null
  return { name: held.plan, seats: held.seats }
})

onMounted(() => {
  void loadAccount()
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
      <SettingsSection :label="t('settings.sec.account')">
        <SettingsRow :title="account?.email ?? ''" :note="t('account.signedInAs')">
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

      <SettingsSection :label="t('account.devices')" :note="t('account.devicesNote')">
        <SettingsRow
          v-for="session in sessions"
          :key="session.id"
          :title="describe(session.device) || t('account.devices')"
          :note="[when(session.createdAt), session.ip].filter(Boolean).join(' · ')"
        >
          <ValueChip v-if="session.id === thisSession" :value="t('account.thisDevice')" />
          <Button v-else variant="outline" size="field" @click="endSession(session.id)">
            {{ t('account.end') }}
          </Button>
        </SettingsRow>
        <SettingsRow
          v-if="sessions.length === 0"
          :title="t('account.devicesEmpty')"
          :note="sessionsFailed ? t('account.devicesFailed') : ''"
        />
        <SettingsRow
          v-if="sessions.length > 1"
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
          <Button variant="danger" size="field" @click="leaving = true">
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
