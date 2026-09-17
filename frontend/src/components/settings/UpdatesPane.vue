<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import Segment from '../ui/Segment.vue'
import { Button } from '../ui/button'
import { Switch } from '../ui/switch'
import UpdateCheck from '../update/UpdateCheck.vue'
import { UpdateChannel } from '../../../bindings/json-inspector/internal/domain'
import { SystemService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useSettings } from '../../composables/useSettings'
import { useUpdateCheck } from '../../composables/useUpdateCheck'
import { formatVersion } from '../../lib/format'
import { formatCheckedAt, useMessages } from '../../i18n'

// Which releases the app may offer, whether it may look for them on its own, and what it is right
// now: the version, and the one button that asks.
const { t } = useMessages()
const { settings, loadSettings, setUpdateCheck, setUpdateChannel } = useSettings()
const { checkedAt, openWindow } = useUpdateCheck()

const version = ref('')
const build = ref('')
const appName = ref('')

const channels = computed(() => [
  { value: UpdateChannel.ChannelStable, label: t('settings.channelStable') },
  { value: UpdateChannel.ChannelBeta, label: t('settings.channelBeta') },
])

// The stored choice until the window has read it: the same defaults Go answers with.
const channel = computed(() => settings.value?.updateChannel ?? UpdateChannel.ChannelStable)
const autoCheck = computed(() => settings.value?.updateCheckAuto ?? true)

// What the app is, in the row that is about it. A call that failed leaves it blank, which is what a
// build stamped by hand shows anyway.
const buildLine = computed(() => t('settings.build', { build: build.value || '—' }))

// The date of the last check sits under the switch rather than beside the button: it describes how
// the app behaves on its own, and that is what the switch decides.
const lastChecked = computed(() =>
  checkedAt.value ? t('update.lastChecked', { at: formatCheckedAt(checkedAt.value) }) : ''
)

onMounted(async () => {
  void loadSettings()
  try {
    version.value = (await SystemService.Version()) ?? ''
    build.value = (await SystemService.Build()) ?? ''
    appName.value = (await SystemService.Name()) ?? ''
  } catch {
    // Nothing to do: the row states the build it can and no more.
  }
})
</script>

<template>
  <div class="pane">
    <SettingsSection :label="t('settings.sec.version')">
      <SettingsRow
        :title="[appName, formatVersion(version)].filter(Boolean).join(' ') || t('settings.title')"
        :note="buildLine"
      >
        <UpdateCheck inline />
      </SettingsRow>
      <SettingsRow :title="t('settings.whatsNew')" :note="t('settings.whatsNewNote')">
        <Button size="field" @click="openWindow()">{{ t('settings.whatsNewOpen') }}</Button>
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.channel')">
      <SettingsRow :title="t('settings.channel')">
        <Segment
          size="sm"
          :grow="false"
          :options="channels"
          :value="channel"
          @pick="setUpdateChannel($event as UpdateChannel)"
        />
      </SettingsRow>
      <SettingsRow :title="t('settings.checkAutomatically')" :note="lastChecked">
        <Switch
          :model-value="autoCheck"
          :aria-label="t('settings.checkAutomatically')"
          @update:model-value="setUpdateCheck(!!$event)"
        />
      </SettingsRow>
    </SettingsSection>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.pane {
  @apply flex flex-col;
  gap: 18px;
}
</style>
