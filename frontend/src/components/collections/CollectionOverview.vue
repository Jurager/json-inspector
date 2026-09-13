<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import CollectionScripts from './CollectionScripts.vue'
import { useCollectionsStore } from '../../stores/collections'
import { useToast } from '../../composables/useToast'
import { requestCount } from '../../lib/collectionTree'
import { formatMicros, plural, statusBadgeClass } from '../../lib/format'
import type { CollectionNode } from '../../../bindings/json-inspector/internal/domain'

const store = useCollectionsStore()
const toast = useToast()

// The header's two panes: what is inside the collection, and the code that runs around its requests.
// Which one is open is the window's, not Go's — the code itself lives in the store, because it is Go's.
const tab = ref<'requests' | 'scripts'>('requests')

// What is being exported is what is selected: a folder exports its subtree, a collection everything
// in it. Go reads the tree again — the rows the list carries have no bodies.
async function exportSelected() {
  const written = await store.exportFile(store.selectedId ?? '')
  if (written) toast.show('Коллекция сохранена в файл')
}

async function importCollection() {
  try {
    const name = await store.importFile()
    if (name) toast.show(`Импортировано: «${name}»`)
  } catch (error) {
    toast.show(`Не удалось импортировать: ${String(error)}`, 'error')
  }
}

const title = computed(() => store.selected?.name ?? store.trail?.collection.name ?? '')
const description = computed(() => store.selected?.description ?? store.trail?.collection.description ?? '')
const requestTotal = computed(() => store.selectedRequestCount)

// The rows of the last run, each with the name and method the tree knows: a result keeps the node id
// and not the node, so what the row is called now is the tree's answer.
interface RunRow {
  nodeId: string
  // What the row opens: the record this request produced. Empty for one that never went out.
  recordId: string
  position: number
  status: number | null
  ok: boolean
  durationUs: number
  error: string
  name: string
  method: string
}

const rows = computed<RunRow[]>(() => {
  const run = store.lastRun
  if (!run) return []
  const byId = new Map<string, CollectionNode>()
  const walk = (nodes: CollectionNode[]) => {
    for (const node of nodes) {
      byId.set(node.id, node)
      walk(node.items ?? [])
    }
  }
  for (const collection of store.tree) walk(collection.items ?? [])

  return [...(run.results ?? [])]
    .sort((a, b) => a.position - b.position)
    .map((result) => ({
      nodeId: result.nodeId,
      recordId: result.recordId ?? '',
      position: result.position,
      status: result.status ?? null,
      ok: result.ok,
      durationUs: result.durationUs,
      error: result.error ?? '',
      name: byId.get(result.nodeId)?.name ?? 'удалённый запрос',
      method: byId.get(result.nodeId)?.method ?? '',
    }))
})

const runName = computed(() => title.value)

function startedAt(run: { startedAt: number }): string {
  return new Date(run.startedAt).toLocaleString('ru-RU', {
    day: 'numeric',
    month: 'long',
    hour: '2-digit',
    minute: '2-digit',
  })
}

async function run() {
  await store.run(store.runNodeId, runName.value)
}

function pluralRequests(n: number): string {
  return `${n} ${plural(n, ['запрос', 'запроса', 'запросов'])}`
}
</script>

<template>
  <div class="overview">
    <div class="head">
      <div class="head-line">
        <span class="title">{{ title }}</span>
        <span class="count">{{ pluralRequests(requestTotal) }}</span>
      </div>
      <div class="head-line second">
        <span class="description">{{ description }}</span>
        <span v-if="store.lastRun" class="last-run">
          Последний прогон {{ startedAt(store.lastRun) }}
        </span>
      </div>

      <div class="actions">
        <Button v-if="!store.running" variant="primary" :disabled="requestTotal === 0" @click="run">
          <Icon name="play" :size="13" /> Запустить коллекцию
        </Button>
        <Button v-else @click="store.stop">
          <Icon name="stop" :size="13" /> Остановить
        </Button>

        <Button :disabled="store.running !== null" @click="importCollection">
          <Icon name="download" :size="13" /> Импорт
        </Button>
        <Button :disabled="store.running !== null || requestTotal === 0" @click="exportSelected">
          <Icon name="upload" :size="13" /> Экспорт
        </Button>
      </div>
    </div>

    <Tabs v-model="tab" class="tabs-host">
      <TabsList class="tabs">
        <TabsTrigger class="tab" value="requests">Запросы</TabsTrigger>
        <TabsTrigger class="tab" value="scripts">Скрипты</TabsTrigger>
      </TabsList>

      <div class="pane">
        <TabsContent value="requests" class="pane-content">
          <div v-if="store.running" class="running">
            Прогон: {{ store.running.done }} / {{ store.running.total || requestTotal }} ·
            {{ store.running.name }}
          </div>

          <template v-if="store.lastRun">
            <div class="summary">
              <div class="cell">
                <span class="cell-value">{{ rows.length }}</span>
                <span class="cell-label">{{ plural(rows.length, ['запрос', 'запроса', 'запросов']) }}</span>
              </div>
              <div class="cell">
                <span class="cell-value ok">{{ store.lastRun.passed }}</span>
                <span class="cell-label">успешно</span>
              </div>
              <div class="cell">
                <span class="cell-value bad">{{ store.lastRun.failed }}</span>
                <span class="cell-label">{{ plural(store.lastRun.failed, ['ошибка', 'ошибки', 'ошибок']) }}</span>
              </div>
              <div class="cell">
                <span class="cell-value">{{ formatMicros(store.lastRun.durationUs) }}</span>
                <span class="cell-label">всего</span>
              </div>
            </div>

            <ul class="results">
              <li
                v-for="row in rows"
                :key="row.nodeId"
                class="result"
                :class="{ failed: !row.ok }"
                @click="store.openRunResult(row)"
              >
                <span class="result-icon" :class="row.ok ? 'ok' : 'bad'">
                  <Icon :name="row.ok ? 'check' : 'xmark'" :size="12" :stroke-width="2.5" />
                </span>
                <span class="badge badge-method result-method">{{ row.method }}</span>
                <span class="result-name" :title="row.error || row.name">{{ row.name }}</span>
                <span v-if="row.status !== null" class="badge" :class="statusBadgeClass(row.status)">
                  {{ row.status }}
                </span>
                <span v-else class="result-error">{{ row.error }}</span>
                <span class="result-time mono">{{ formatMicros(row.durationUs) }}</span>
              </li>
            </ul>
          </template>

          <div v-else-if="!store.running" class="idle">
            <span class="idle-title">Ещё не запускали</span>
            <span>Прогон отправит все запросы по очереди и покажет, что ответил каждый.</span>
          </div>
        </TabsContent>

        <TabsContent value="scripts">
          <CollectionScripts />
        </TabsContent>
      </div>
    </Tabs>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.overview {
  @apply flex-1 min-h-0 flex flex-col;
}

/* The tabs run the width of the panel, so the padding that used to be here belongs to the two blocks
   around them: what is above the tabs, and what each tab draws. */
.head {
  @apply flex flex-col gap-1 px-6 pt-5 pb-4;
}

.tabs-host {
  @apply flex-1 min-h-0 flex flex-col gap-0;
}

.pane {
  @apply flex-1 min-h-0 overflow-y-auto;
}

.pane-content {
  @apply px-6 py-5;
}

.head-line {
  @apply flex items-baseline gap-2.5;
}

.head-line.second {
  @apply justify-between;
}

.title {
  @apply text-[20px] font-semibold;
}

.count {
  @apply text-[11.5px] text-text-tertiary;
}

.description {
  @apply text-[13px] text-text-secondary;
}

.last-run {
  @apply text-[11.5px] text-text-tertiary;
}

.actions {
  @apply flex items-center gap-2 mt-4;
}

.running {
  @apply mt-3 text-[12.5px] text-text-secondary;
}

.summary {
  @apply grid grid-cols-4 gap-2 mt-5;
}

.cell {
  @apply flex flex-col gap-0.5 px-3 py-2.5 rounded-lg border border-border;
  background: var(--bg-inset);
}

.cell-value {
  @apply text-[15px] font-semibold;
}

.cell-value.ok {
  @apply text-green;
}

.cell-value.bad {
  @apply text-red;
}

.cell-label {
  @apply text-[11px] text-text-tertiary;
}

.results {
  @apply flex flex-col mt-4;
}

.result {
  @apply flex items-center gap-2 h-[30px] px-2 rounded-md text-[12.5px] cursor-pointer;
}

.result:hover {
  @apply bg-bg-hover;
}

/* A row that failed is tinted the way the design tints it, so a wall of green and red is readable at
   a glance. */
.result.failed {
  background: rgba(255, 59, 48, 0.04);
}

.result-icon {
  @apply flex-none inline-flex items-center justify-center w-4;
}

.result-icon.ok {
  @apply text-green;
}

.result-icon.bad {
  @apply text-red;
}

.result-method {
  @apply flex-none;
  min-width: 40px;
}

.result-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.result-error {
  @apply flex-none max-w-[320px] overflow-hidden text-ellipsis whitespace-nowrap text-red text-[11.5px];
}

.result-time {
  @apply flex-none text-[11.5px] text-text-tertiary;
}

.idle {
  @apply flex flex-col gap-1 mt-6 text-[12.5px] text-text-tertiary;
}

.idle-title {
  @apply font-semibold text-text-secondary text-[13px];
}
</style>
