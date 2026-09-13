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
import { formatBytes, formatVersion } from '../../lib/format'
import { useEnvironmentsStore } from '../../stores/environments'
import { useCollectionsStore } from '../../stores/collections'
import type { Info as UpdateInfo } from '../../../bindings/json-inspector/internal/infra/updater'

const props = defineProps<{ updateInfo: UpdateInfo | null }>()
const emit = defineEmits<{ (e: 'open-update'): void }>()

const store = useRequestsStore()
const collections = useCollectionsStore()
const envStore = useEnvironmentsStore()

const environmentName = computed(() => envStore.activeEnvironment?.name ?? 'Без окружения')

const missingCount = computed(() => {
  if (store.activeView === 'request') return store.missingVars.length
  if (store.activeView === 'collections' && collections.cardOpen) return collections.missingVars.length
  return 0
})

function plural(n: number, forms: [string, string, string]): string {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return forms[0]
  if (m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14)) return forms[1]
  return forms[2]
}

const missingLabel = computed(
  () => `${missingCount.value} ${plural(missingCount.value, ['переменная', 'переменные', 'переменных'])} не найдено`
)

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
    parts.push(`${total} ресурсов`)
    if (missing > 0) parts.push(`${missing} связи не загружены`)
    return parts.join(' · ')
  }
  const ct = r.contentType || ''
  const size = formatBytes(new Blob([r.responseBody]).size)
  return ct ? `${ct} · ${size}` : size
})

// The run of a collection is what the left side says while it lasts: it is the same slot the
// environment and the capture state use, and no two views are on screen at once.
const runLabel = computed(() => {
  const run = collections.running
  if (!run) return ''
  return `Прогон: ${run.done} / ${run.total || collections.selectedRequestCount} · ${run.name}`
})

const capture = computed(() => store.capture)

const captureLabel = computed(() => {
  const c = capture.value
  if (c.recording) return `Запись · ${c.tabs} вкладок под перехватом`
  if (c.connected) return 'Расширение подключено'
  return 'Перехват не запущен · ожидание расширения'
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
        <span class="crumbs">
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
      <span class="unsaved">Не сохранено</span>
      <button class="save-link" @click="collections.saveNode()">Сохранить</button>
      <span class="divider"></span>
    </template>

    <button v-if="updateInfo" class="update-link" @click="emit('open-update')">
      Доступна версия {{ formatVersion(updateInfo.latest) }}
    </button>

    <span v-if="summary" class="summary">{{ summary }}</span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.status-bar {
  @apply flex-none flex items-center gap-2.5 h-7 px-3.5 text-xs;
  border-top: 1px solid var(--border);
  background: var(--bg-sidebar);
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

/* The last crumb is what is open, so it reads as the title of the pane rather than as part of a path. */
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
