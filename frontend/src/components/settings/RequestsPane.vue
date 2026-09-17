<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import SettingsRow from './SettingsRow.vue'
import SettingsSection from './SettingsSection.vue'
import ClearHistorySheet from './ClearHistorySheet.vue'
import CaptureFiltersSheet from '../browser/CaptureFiltersSheet.vue'
import Segment from '../ui/Segment.vue'
import { Button } from '../ui/button'
import { Retention } from '../../../bindings/json-inspector/internal/domain'
import { RecordsService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useSettings } from '../../composables/useSettings'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, formatBytes, useMessages } from '../../i18n'

// What the app keeps of what went through it: how long history is held, and which traffic from the
// browser is worth keeping in the first place. Both are rules rather than geometry — neither follows
// a drag — and both are the only two settings the app has about requests at all.
const { t } = useMessages()
const { settings, loadSettings, setRetention } = useSettings()
const { setNotice, clearNotice } = useSheetNotice()

const retentions = computed(() => [
  { value: Retention.RetainWeek, label: t('settings.retentionWeek') },
  { value: Retention.RetainMonth, label: t('settings.retentionMonth') },
  { value: Retention.RetainForever, label: t('settings.retentionForever') },
])

const retention = computed(() => settings.value?.historyRetention ?? Retention.RetainForever)

// What the space on screen is holding, read here rather than in Go's snapshot: it changes with every
// request that goes out, and the settings window is the one place it is shown.
const count = ref(0)
const bytes = ref(0)
const clearing = ref(false)
const filtersOpen = ref(false)

const held = computed(() => t('counts.requests', count.value))
const size = computed(() => formatBytes(bytes.value))

async function readHistory() {
  try {
    const stats = await RecordsService.History()
    count.value = stats?.count ?? 0
    bytes.value = stats?.bytes ?? 0
  } catch (error) {
    setNotice(describeFailure(error))
  }
}

// How the capture rules read as one line: the hosts that are kept, and the kinds of traffic that are
// dropped before they ever reach the list. A rule that is off says nothing — the sentence is about
// what is being captured, and listing the four switches would be a second copy of the sheet.
const rules = computed(() => {
  const filters = settings.value?.captureFilters
  if (!filters) return t('settings.captureNone')
  const hosts = filters.hosts?.length ? filters.hosts.join(', ') : t('settings.captureAllHosts')
  const dropped = [
    filters.static ? t('browser.ruleStatic') : '',
    filters.analytics ? t('browser.ruleAnalytics') : '',
    filters.json ? t('browser.ruleJson') : '',
  ].filter(Boolean)
  return dropped.length
    ? t('settings.captureDropped', { hosts, rules: dropped.join(', ') })
    : t('settings.captureKept', { hosts })
})

async function changeRetention(value: string) {
  clearNotice()
  await setRetention(value as Retention)
  // The window that shortened it is the one that has to show the result: the records the rule just
  // dropped are exactly what the line beside this segment counts.
  await readHistory()
}

function cleared() {
  clearing.value = false
  void readHistory()
}

onMounted(() => {
  void loadSettings().then(readHistory)
})
</script>

<template>
  <div class="pane">
    <SettingsSection :label="t('settings.sec.history')" :note="t('settings.historyNote')">
      <SettingsRow :title="t('settings.retention')">
        <Segment size="sm"
          :grow="false" :options="retentions" :value="retention" @pick="changeRetention" />
      </SettingsRow>
      <SettingsRow :title="t('settings.clearHistory')" :note="`${held} · ${size}`">
        <Button variant="danger" size="field" @click="clearing = true">
          {{ t('settings.clearAction') }}
        </Button>
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :label="t('settings.sec.capture')" :note="t('settings.captureNote')">
      <SettingsRow :title="t('settings.captureRules')" :note="rules">
        <Button size="field" @click="filtersOpen = true">{{ t('settings.captureEdit') }}</Button>
      </SettingsRow>
    </SettingsSection>

    <ClearHistorySheet
      v-if="clearing"
      :count="count"
      :bytes="bytes"
      @close="clearing = false"
      @cleared="cleared"
    />

    <CaptureFiltersSheet v-if="filtersOpen" @close="filtersOpen = false" @saved="filtersOpen = false" />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.pane {
  @apply flex flex-col;
  gap: 18px;
}
</style>
