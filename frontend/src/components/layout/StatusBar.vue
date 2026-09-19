<script setup lang="ts">
import { computed } from 'vue'
import { useRequestsStore } from '../../stores/requests'
import {
  dataResources,
  documentVersion,
  isJsonApi,
  missingLinkCount,
  type JsonApiDocument,
} from '../../lib/jsonapi'
import { tryParseJson } from '../../lib/json'
import { formatBytes, formatMicros, formatNumber, useMessages } from '../../i18n'
import { formatVersion } from '../../lib/format'
import { useEnvironmentsStore } from '../../stores/environments'
import { useCollectionsStore } from '../../stores/collections'
import { useSettings } from '../../composables/useSettings'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import { ListSide } from '../../../bindings/json-inspector/internal/domain'
import type { Info as UpdateInfo } from '../../../bindings/json-inspector/internal/usecase/update'

const props = defineProps<{ updateInfo: UpdateInfo | null }>()
const emit = defineEmits<{ (e: 'open-update'): void }>()

const { t } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()
const { settings, setLayout } = useSettings()

// Where the list panel will be, which is not the same question as whether it is on screen: an empty
// list is not drawn at all, and these three still say where it goes when there is something in it.
const PANEL_SIDES: { side: ListSide; icon: string; hint: string }[] = [
  { side: ListSide.ListSideLeft, icon: 'panel-left', hint: 'status.listLeft' },
  { side: ListSide.ListSideRight, icon: 'panel-right', hint: 'status.listRight' },
  { side: ListSide.ListSideHidden, icon: 'panel-hidden', hint: 'status.listHidden' },
]

const listSide = computed<ListSide>(() => settings.value?.listSide ?? ListSide.ListSideLeft)

const environmentName = computed(() => envStore.activeEnvironment?.name ?? t('titlebar.noEnvironment'))

const missingCount = computed(() => {
  if (store.activeView === 'request') return store.missingVars.length
  if (store.activeView === 'collections' && collections.cardOpen) return collections.missingVars.length
  return 0
})

const missingLabel = computed(() => t('counts.variablesMissing', missingCount.value))

const selectedRecord = computed(() => {
  if (store.activeView === 'request') return store.manualSelected
  if (store.activeView === 'browser') return store.browserSelected
  if (store.activeView === 'collections') return collections.response
  return null
})

const doc = computed<JsonApiDocument | null>(() => {
  const r = selectedRecord.value
  if (!r) return null
  const p = tryParseJson(r.responseBody)
  return p.ok && isJsonApi(p.value) ? (p.value as JsonApiDocument) : null
})

const summary = computed(() => {
  const r = selectedRecord.value
  if (!r) return ''
  if (doc.value) {
    const d = doc.value
    const version = documentVersion(d)
    const total = dataResources(d).length + (d.included ?? []).length
    const missing = missingLinkCount(d)
    const parts: string[] = []
    if (version) parts.push(`JSON:API ${version}`)
    parts.push(t('counts.resources', total))
    if (missing > 0) parts.push(t('counts.linksMissing', missing))
    return parts.join(' · ')
  }
  const ct = r.contentType || ''
  const size = formatBytes(new Blob([r.responseBody]).size)
  return ct ? `${ct} · ${size}` : size
})

const runLabel = computed(() => {
  const run = collections.running
  if (!run) return ''
  const total = run.total || collections.selectedRequestCount
  return t('status.runRunning', { done: formatNumber(run.done), total: formatNumber(total), name: run.name })
})

// The whole line is one message: it is a sentence, and a sentence is not assembled out of pieces if a
// translation is allowed to put them in its own order.
const runOutcome = computed(() => {
  if (store.activeView !== 'collections' || collections.cardOpen) return ''
  const run = collections.lastRun
  if (!run || collections.running) return ''
  return t('status.runFinished', {
    passed: formatNumber(run.passed),
    failed: t('counts.errors', run.failed),
    duration: formatMicros(run.durationUs),
  })
})

const capture = computed(() => store.capture)

// Whether anything already stands to the right of the spacer, and so whether the three buttons need
// a rule in front of them: they are the last thing in the bar and the only thing that can be there
// on its own.
const rightSide = computed(
  () =>
    Boolean(summary.value || runOutcome.value || props.updateInfo) ||
    (store.activeView === 'collections' && collections.dirty)
)

const captureLabel = computed(() => {
  const c = capture.value
  if (c.recording) return t('status.captureRecording', { tabs: t('counts.tabs', c.tabs) })
  if (c.connected) return t('status.captureConnected')
  return t('status.captureIdle')
})

const captureDotClass = computed(() => {
  const c = capture.value
  if (c.recording) return 'dot dot-green'
  if (c.connected) return 'dot dot-grey'
  return 'dot dot-orange'
})

// The switch is the store's: the browser page offers the same one, and two copies of it would be two
// answers to the same question.
const toggleCapture = () => store.toggleCapture()
</script>

<template>
  <div class="status-bar">
    <template v-if="store.activeView === 'request'">
      <span>{{ environmentName }}</span>
      <template v-if="missingCount > 0">
        <span class="divider"></span>
        <span class="missing">{{ missingLabel }}</span>
      </template>
    </template>
    <template v-else-if="store.activeView === 'browser'">
      <span :class="captureDotClass"></span>
      <span>{{ captureLabel }}</span>
      <!-- Nothing to hold down while the extension is away, so the switch is not there to hold. -->
      <template v-if="capture.recording || capture.paused">
        <span class="divider"></span>
        <IconButton
          size="md"
          :hint="capture.paused ? t('status.resumeCapture') : t('status.pauseCapture')"
          @click="toggleCapture"
        >
          <Icon
            :name="capture.paused ? 'play' : 'pause'"
            :size="15"
            filled
            :class="{ paused: capture.paused }"
          />
        </IconButton>
      </template>
    </template>
    <template v-else-if="store.activeView === 'collections'">
      <template v-if="collections.running">
        <span class="dot dot-orange"></span>
        <span>{{ runLabel }}</span>
      </template>
      <template v-else-if="collections.selectedId">
        <!-- A folder's name is the only thing here: a request's path is written above the request
             itself, in the bar that says what the pane below is a picture of. -->
        <span v-if="!collections.cardOpen" class="crumb-last">{{ collections.levelName }}</span>
        <template v-if="missingCount > 0">
          <span class="divider"></span>
          <span class="missing">{{ missingLabel }}</span>
        </template>
      </template>
    </template>

    <span class="spacer"></span>

    <template v-if="store.activeView === 'collections' && collections.dirty">
      <span class="dot dot-orange"></span>
      <span class="unsaved">{{ t('status.unsaved') }}</span>
      <button class="save-link" @click="collections.saveNode()">{{ t('common.save') }}</button>
      <span class="divider"></span>
    </template>

    <button v-if="updateInfo" class="update-link" @click="emit('open-update')">
      {{ t('status.updateAvailable', { version: formatVersion(updateInfo.latest) }) }}
    </button>

    <span v-if="runOutcome" class="summary">{{ runOutcome }}</span>
    <span v-else-if="summary" class="summary">{{ summary }}</span>

    <!-- The list panel's place, in every section and whether or not the panel is drawn: a list with
         nothing in it is not shown at all, and the choice made here is kept for it. -->
    <span v-if="rightSide" class="divider"></span>
    <IconButton
      v-for="side in PANEL_SIDES"
      :key="side.side"
      size="sm"
      :hint="t(side.hint)"
      @click="setLayout({ listSide: side.side })"
    >
      <Icon
        :name="side.icon"
        :size="12"
        :stroke-width="1.6"
        :class="{ current: listSide === side.side }"
      />
    </IconButton>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The third chrome surface: the same glass as the titlebar and the rail. */
.status-bar {
  @apply relative flex-none flex items-center gap-2.5 h-[var(--statusbar-height)] px-4 text-[12px];
  border-top: 1px solid var(--glass-chrome-border);
  /* The bottom strip mirrors the top one: the colour comes up from the window's edge and is gone by
     the time the bar ends. */
  background-image:
    linear-gradient(var(--glass-chrome), var(--glass-chrome)),
    linear-gradient(0deg, var(--tint-near) 0%, var(--tint-far) 100%);
  transition: background-image 0.35s ease;
  backdrop-filter: var(--blur-chrome);
  color: var(--text-secondary);
}


.spacer {
  @apply flex-1;
}

.missing {
  @apply text-red;
}

.divider {
  @apply w-px h-3 bg-border flex-none;
}

.dot {
  @apply w-[7px] h-[7px] rounded-full flex-none;
}

.dot-red {
  background: var(--red);
}

.dot-green {
  background: var(--green);
}

.dot-grey {
  background: var(--text-tertiary);
}

.dot-orange {
  background: var(--orange);
}

.update-link {
  @apply text-accent bg-transparent border-none cursor-pointer p-0 text-xs;
  font: inherit;
}

.update-link:hover {
  text-decoration: underline;
}

.summary {
  @apply whitespace-nowrap;
}

/* The switch is the only place capture is held, so while it is held the button says so in the
   colour the paused state carries everywhere else in the window. */
.status-bar .paused {
  color: var(--orange);
}

/* The three positions are drawn alike, and the one in force is at full strength: the trio reads as
   a switch with a handle rather than as three commands. The colour goes on the glyph, where it is a
   property of the element itself and not an inheritance the button's own variant can overrule. */
.status-bar .current {
  @apply text-text;
}

.crumb-last {
  @apply text-text font-semibold overflow-hidden text-ellipsis whitespace-nowrap;
}

.unsaved {
  @apply text-text-secondary;
}

.save-link {
  @apply text-accent bg-transparent border-none cursor-pointer p-0 text-xs;
  font: inherit;
}

.save-link:hover {
  text-decoration: underline;
}
</style>
