<script setup lang="ts">
import { computed } from 'vue'
import { useRequestsStore } from '../../stores/requests'
import {
  buildIndex,
  dataResources,
  isJsonApi,
  relIdentifiers,
  resourceKey,
  type JsonApiDocument,
} from '../../lib/jsonapi'
import { tryParseJson, formatBytes } from '../../lib/json'
import { useEnvironmentsStore } from '../../stores/environments'

interface UpdateInfo {
  available: boolean
  current: string
  latest: string
}

const props = defineProps<{ update: UpdateInfo | null }>()
const emit = defineEmits<{ (e: 'open-update'): void }>()

const store = useRequestsStore()
const envStore = useEnvironmentsStore()

const environment = computed(() => envStore.active?.name ?? 'Без окружения')

const missingCount = computed(() =>
  store.activeView === 'request' ? store.missingVars.length : 0
)

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

const selected = computed(() =>
  store.activeView === 'request' ? store.manualSelected : store.browserSelected
)

const doc = computed<JsonApiDocument | null>(() => {
  const r = selected.value
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

// Counts relationship references whose target isn't present in data + included
// — the "незагруженные связи" shown in the summary.
function countMissing(d: JsonApiDocument): number {
  const idx = buildIndex(d)
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
  const r = selected.value
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
      <span>{{ environment }}</span>
      <template v-if="missingCount > 0">
        <span class="divider"></span>
        <span class="missing">{{ missingLabel }}</span>
      </template>
    </template>
    <template v-else>
      <span :class="captureDotClass"></span>
      <span>{{ captureLabel }}</span>
    </template>

    <span class="spacer"></span>

    <button v-if="update" class="update-link" @click="emit('open-update')">
      Доступна версия {{ update.latest }}
    </button>

    <span v-if="summary" class="summary">{{ summary }}</span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.status-bar {
  @apply flex-none flex items-center gap-2.5 h-7 px-3.5 text-[11.5px];
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
  @apply text-accent bg-transparent border-none cursor-pointer p-0 text-[11.5px];
  font: inherit;
}

.update-link:hover {
  text-decoration: underline;
}

.summary {
  @apply whitespace-nowrap;
}
</style>
