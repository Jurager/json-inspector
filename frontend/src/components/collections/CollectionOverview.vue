<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/tabs'
import CollectionAuth from './CollectionAuth.vue'
import CollectionScripts from './CollectionScripts.vue'
import { useCollectionsStore } from '../../stores/collections'
import { useToast } from '../../composables/useToast'
import { requestCount } from '../../lib/collectionTree'
import { formatAgo, formatMicros, plural } from '../../lib/format'
import type { CollectionNode } from '../../../bindings/json-inspector/internal/domain'

const store = useCollectionsStore()
const toast = useToast()

const tab = ref<'requests' | 'auth' | 'scripts'>('requests')

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
const editingDescription = ref(false)
const descriptionDraft = ref('')
const descriptionInput = ref<HTMLInputElement | null>(null)
const levelId = computed(() => store.selected?.id ?? store.trail?.collection.id ?? '')
const descriptionLevel = ref('')
const descriptionOpen = ref('')

function editDescription() {
  descriptionDraft.value = description.value
  descriptionLevel.value = levelId.value
  descriptionOpen.value = description.value
  editingDescription.value = true
  nextTick(() => {
    descriptionInput.value?.focus()
    descriptionInput.value?.select()
  })
}

async function commitDescription() {
  if (!editingDescription.value) return
  editingDescription.value = false
  const written = descriptionDraft.value.trim()
  if (!descriptionLevel.value || written === descriptionOpen.value) return
  await store.describe(descriptionLevel.value, written)
}

function onDescriptionKeydown(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    void commitDescription()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    editingDescription.value = false
  }
}

watch(levelId, () => {
  editingDescription.value = false
})

interface RunRow {
  nodeId: string
  recordId: string
  position: number
  status: number | null
  ok: boolean
  skipped: boolean
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
      skipped: result.skipped ?? false,
      durationUs: result.durationUs,
      error: result.error ?? '',
      name: byId.get(result.nodeId)?.name ?? 'удалённый запрос',
      method: byId.get(result.nodeId)?.method ?? '',
    }))
})

const runName = computed(() => title.value)
const passed = computed(() => rows.value.filter((row) => row.ok).length)
const failed = computed(() => rows.value.filter((row) => !row.ok && !row.skipped).length)
const lastRunAt = computed(() => store.lastRun?.finishedAt || store.lastRun?.startedAt || 0)

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

      <input
        v-if="editingDescription"
        ref="descriptionInput"
        v-model="descriptionDraft"
        class="description-input"
        placeholder="Добавить описание"
        spellcheck="false"
        @keydown="onDescriptionKeydown"
        @blur="commitDescription"
      />
      <span
        v-else
        class="description"
        :class="{ placeholder: !description }"
        role="button"
        tabindex="0"
        :title="description || 'Добавить описание'"
        @click="editDescription"
        @keydown.enter="editDescription"
      >
        {{ description || 'Добавить описание' }}
      </span>

      <div class="actions">
        <Button
          v-if="!store.running"
          variant="primary"
          size="lg"
          :disabled="requestTotal === 0"
          @click="run"
        >
          <Icon name="play" :size="11" /> Запустить коллекцию
        </Button>
        <Button v-else size="lg" @click="store.stop">
          <span class="spinner spinner-sm"></span> Остановить
        </Button>

        <Button size="lg" :disabled="store.running !== null" @click="importCollection">
          <Icon name="download" :size="12" /> Импорт
        </Button>
        <Button
          size="lg"
          :disabled="store.running !== null || requestTotal === 0"
          @click="exportSelected"
        >
          <Icon name="upload" :size="12" /> Экспорт
        </Button>

        <span v-if="store.lastRun && !store.running" class="last-run">
          Прогон {{ formatAgo(lastRunAt) }}
        </span>
      </div>
    </div>

    <Tabs v-model="tab" class="tabs-host">
      <TabsList class="tabs coll-tabs">
        <TabsTrigger class="tab coll-tab" value="requests">Запросы</TabsTrigger>
        <TabsTrigger class="tab coll-tab" value="auth">Авторизация</TabsTrigger>
        <TabsTrigger class="tab coll-tab" value="scripts">Скрипты</TabsTrigger>
      </TabsList>

      <div class="pane">
        <TabsContent value="requests" class="tab-pane">
          <div v-if="store.lastRun" class="summary">
            <div class="cell">
              <span class="cell-value">{{ rows.length }}</span>
              <span class="cell-label">{{ plural(rows.length, ['запрос', 'запроса', 'запросов']) }}</span>
            </div>
            <div class="cell">
              <span class="cell-value ok">{{ passed }}</span>
              <span class="cell-label">успешно</span>
            </div>
            <div class="cell">
              <span class="cell-value bad">{{ failed }}</span>
              <span class="cell-label">{{ plural(failed, ['ошибка', 'ошибки', 'ошибок']) }}</span>
            </div>
            <div class="cell">
              <span class="cell-value">
                {{ store.running ? '—' : formatMicros(store.lastRun.durationUs) }}
              </span>
              <span class="cell-label">общее время</span>
            </div>
          </div>

          <div class="results-area">
            <div v-if="store.lastRun" class="results">
              <div
                v-for="row in rows"
                :key="row.nodeId"
                class="result"
                :class="{ failed: !row.ok }"
                @click="store.openRunResult(row)"
              >
                <span class="result-icon" :class="row.ok ? 'ok' : 'bad'">
                  <Icon :name="row.ok ? 'check' : 'xmark'" :size="13" :stroke-width="2.2" />
                </span>
                <span class="result-method mono">{{ row.method }}</span>
                <span class="result-name" :title="row.error || row.name">{{ row.name }}</span>
                <span v-if="row.status !== null" class="result-status" :class="row.ok ? 'ok' : 'bad'">
                  {{ row.status }}
                </span>
                <span v-else class="result-error">{{ row.error }}</span>
                <span class="result-time">{{ formatMicros(row.durationUs) }}</span>
              </div>
            </div>

            <div v-else-if="!store.running" class="idle">
              <span class="idle-title">Ещё не запускали</span>
              <span>Прогон отправит все запросы по очереди и покажет, что ответил каждый.</span>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="auth" class="tab-pane scripts-pane">
          <CollectionAuth />
        </TabsContent>

        <TabsContent value="scripts" class="tab-pane scripts-pane">
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

.head {
  @apply flex-none flex flex-col gap-1.5 px-6 pt-5 pb-4 border-b border-border bg-bg-panel;
}

.head-line {
  @apply flex items-center gap-2.5;
}

.title {
  @apply text-[20px] font-semibold tracking-[-0.01em];
}

.count {
  @apply text-[11.5px] text-text-tertiary tabular-nums;
}

.description {
  @apply min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13px] text-text-secondary cursor-text;
}

.description.placeholder {
  @apply text-text-tertiary;
}

.description-input {
  @apply min-w-0 p-0 border-none outline-none bg-transparent text-[13px] text-text-secondary;
}

.description-input::placeholder {
  @apply text-text-tertiary;
}

.actions {
  @apply flex items-center gap-2.5 mt-1.5;
}

.last-run {
  @apply ml-auto text-[11.5px] text-text-tertiary;
}

.coll-tabs {
  @apply flex-none h-[38px] gap-0.5 px-6;
}

.coll-tab {
  @apply h-[38px] px-1 mr-5 text-[12.5px] border-b-2;
}

.coll-tab[data-state='active'] {
  @apply text-accent font-medium;
  border-bottom-color: var(--accent);
}

.tabs-host {
  @apply flex-1 min-h-0 flex flex-col gap-0;
}

.pane {
  @apply flex-1 min-h-0 flex flex-col;
}

.tab-pane {
  @apply flex-1 min-h-0 flex flex-col;
}

.scripts-pane {
  @apply overflow-y-auto bg-bg-panel;
}

.summary {
  @apply flex-none flex items-center gap-5 px-6 py-3 border-b border-border bg-bg-panel;
}

.cell {
  @apply flex flex-col gap-px;
}

.cell-value {
  @apply text-[17px] font-semibold;
}

.cell-value.ok {
  color: var(--green-text);
}

.cell-value.bad {
  color: var(--red-text);
}

.cell-label {
  @apply text-[10.5px] text-text-tertiary;
}

.results-area {
  @apply flex-1 min-h-0 overflow-y-auto px-6 py-3;
  background: var(--bg);
}

.results {
  @apply rounded-[9px] border border-border bg-bg-panel overflow-hidden;
}

.result {
  @apply flex items-center gap-2.5 px-3.5 py-2 text-[12.5px] cursor-pointer;
  border-bottom: 1px solid var(--border);
}

.result:last-child {
  border-bottom: none;
}

.result:hover {
  @apply bg-bg-hover;
}

.result.failed {
  background: color-mix(in srgb, var(--red) 4%, transparent);
}

.result-icon {
  @apply flex-none inline-flex items-center justify-center w-[13px];
}

.result-icon.ok {
  color: var(--green-text);
}

.result-icon.bad {
  color: var(--red-text);
}

.result-method {
  @apply flex-none text-[10.5px] font-semibold text-text-secondary;
  min-width: 44px;
}

.result-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-text;
}

.result-status {
  @apply flex-none px-1.5 py-px rounded-[4px] text-[10px] font-semibold tabular-nums;
}

.result-status.ok {
  color: var(--green-text);
  background: var(--green-soft);
}

.result-status.bad {
  color: var(--red-text);
  background: var(--red-soft);
}

.result-error {
  @apply flex-none max-w-[320px] overflow-hidden text-ellipsis whitespace-nowrap text-[11.5px];
  color: var(--red-text);
}

.result-time {
  @apply flex-none w-[52px] text-right text-[10.5px] text-text-tertiary tabular-nums;
}

.idle {
  @apply flex flex-col gap-1 text-[12.5px] text-text-tertiary;
}

.idle-title {
  @apply font-semibold text-text-secondary text-[12.5px];
}
</style>
