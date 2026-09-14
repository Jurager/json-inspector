<script setup lang="ts">
import { computed } from 'vue'
import { useRequestsStore } from '../../stores/requests'
import {
  buildResourceIndex,
  dataResources,
  isJsonApi,
  relIdentifiers,
  resourceKey,
  type JsonApiDocument,
} from '../../lib/jsonapi'
import { tryParseJson } from '../../lib/json'
import { formatBytes, formatMicros, formatNumber, useMessages } from '../../i18n'
import { formatVersion } from '../../lib/format'
import { useEnvironmentsStore } from '../../stores/environments'
import { useCollectionsStore } from '../../stores/collections'
import type { Info as UpdateInfo } from '../../../bindings/json-inspector/internal/infra/updater'

const props = defineProps<{ updateInfo: UpdateInfo | null }>()
const emit = defineEmits<{ (e: 'open-update'): void }>()

const { t } = useMessages()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()

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

function jsonapiVersion(d: JsonApiDocument): string {
  const j = d.jsonapi
  if (j && typeof j === 'object' && !Array.isArray(j)) {
    const v = (j as Record<string, unknown>).version
    if (typeof v === 'string' && v) return v
  }
  return ''
}

function countMissing(d: JsonApiDocument): number {
  const idx = buildResourceIndex(d)
  let n = 0
  for (const r of [...dataResources(d), ...(d.included ?? [])]) {
    for (const rel of Object.values(r.relationships ?? {})) {
      for (const ri of relIdentifiers(rel)) {
        if (!idx.has(resourceKey(ri.type, ri.id))) n++
      }
    }
  }
  return n
}

const summary = computed(() => {
  const r = selectedRecord.value
  if (!r) return ''
  if (doc.value) {
    const d = doc.value
    const version = jsonapiVersion(d)
    const total = dataResources(d).length + (d.included ?? []).length
    const missing = countMissing(d)
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
    </template>
    <template v-else-if="store.activeView === 'collections'">
      <template v-if="collections.running">
        <span class="dot dot-orange"></span>
        <span>{{ runLabel }}</span>
      </template>
      <template v-else-if="collections.selectedId">
        <span v-if="!collections.cardOpen" class="crumb-last">{{ collections.levelName }}</span>
        <span v-else class="crumbs">
          <template v-for="(crumb, i) in collections.breadcrumbs" :key="crumb.id">
            <span v-if="i > 0" class="crumb-sep">›</span>
            <span :class="i === collections.breadcrumbs.length - 1 ? 'crumb-last' : ''">
              {{ crumb.name }}
            </span>
          </template>
        </span>
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
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The third chrome surface: the same glass as the titlebar and the rail. */
.status-bar {
  @apply relative flex-none flex items-center gap-2.5 h-[var(--statusbar-height)] px-3.5 text-xs;
  border-top: 1px solid var(--glass-chrome-border);
  background: var(--glass-chrome);
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

.crumbs {
  @apply flex items-center gap-1.5 min-w-0 overflow-hidden;
}

.crumb-sep {
  @apply text-text-tertiary;
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
