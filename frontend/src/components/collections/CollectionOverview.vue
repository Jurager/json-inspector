<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import CollectionAuth from './CollectionAuth.vue'
import CollectionScripts from './CollectionScripts.vue'
import { Sheet } from '../ui/sheet'
import CollectionVariables from './CollectionVariables.vue'
import { useCollectionsStore } from '../../stores/collections'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import { useToast } from '../../composables/useToast'
import { findNode } from '../../lib/collectionTree'
import { addressOf } from '../../lib/address'
import { methodInkClass, statusBadgeClass } from '../../lib/format'
import { describeFailure, formatAgo, formatMicros, refusalText, useMessages } from '../../i18n'
import type {
  Collection,
  CollectionRun,
  CollectionRunResult,
  LevelRow,
} from '../../../bindings/json-inspector/internal/domain'
import { CollectionsService } from '../../../bindings/json-inspector/internal/transport/wails'

// One collection's page: what it is, what it has come to, and the requests inside it. It is a page
// rather than a set of tabs because the three things a collection owns — its authorization, its
// variables and its scripts — are worth a line each, and a line says more about them at a glance than
// a tab nobody opens does.
const { t } = useMessages()

const store = useCollectionsStore()
const { schemeOf } = useAuthSchemes()
const toast = useToast()

const trail = computed(() => store.trail)
// The page is about a collection: it is drawn when a row is selected and that row is not a request.
const level = computed<Collection | null>(() => trail.value?.collection ?? null)
const levelId = computed(() => level.value?.id ?? '')
const title = computed(() => level.value?.name ?? '')

async function exportLevel() {
  const written = await store.exportFile(levelId.value)
  if (written) toast.show(t('collections.savedToFile'))
}

async function importCollection() {
  try {
    const name = await store.importFile()
    if (name) toast.show(t('collections.imported', { name }))
  } catch (error) {
    toast.show(t('collections.importFailed', { error: describeFailure(error) }), 'error')
  }
}

// ---- the description -----------------------------------------------------

const description = computed(() => level.value?.description ?? '')
const editingDescription = ref(false)
const descriptionDraft = ref('')
const descriptionInput = ref<HTMLInputElement | null>(null)
const descriptionOpen = ref('')

function editDescription() {
  descriptionDraft.value = description.value
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
  if (!levelId.value || written === descriptionOpen.value) return
  await store.describe(levelId.value, written)
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

// ---- the requests of this level ------------------------------------------

// What the table draws comes from two places and neither is the tree: the requests inside the
// collection — the folders' ones included, each with the folder it sits in — and the last run's
// answers to them. The tree deliberately carries no request payload — an address is payload — and
// the run is the only thing that knows what each request answered.
//
// The whole collection and not the level alone, because the table is the report of a run: running a
// collection sends everything inside it, so a table that listed the top level would answer for a
// run nobody made. It is also what the folder column on the rail counts — the same number, drawn
// from the same walk.
//
// The two refs are declared before the watcher below, and not beside the code that fills them: the
// watcher starts by running once, in this same setup, and a `const` reached from there would still
// be in its own dead zone — which is a page that draws itself empty the first time it is opened.
const requests = ref<LevelRow[]>([])
const loading = ref(false)

async function loadRows() {
  if (!levelId.value) {
    requests.value = []
    return
  }
  loading.value = true
  try {
    requests.value = (await CollectionsService.Contents(levelId.value)) ?? []
  } finally {
    loading.value = false
  }
}

// The rows and the code are read when the level opens and again when the selection moves. Neither is
// in the tree, and a page that drew itself empty until a click would be a page that lied once.
watch(
  levelId,
  () => {
    editingDescription.value = false
    void loadRows()
    // A collection's code is not in the tree — it is a table of its own, keyed by level — so what the
    // page says about it is what this reads.
    void store.loadScripts()
  },
  { immediate: true }
)

interface Answer {
  status: number | null
  ok: boolean
  skipped: boolean
  error: string
  ms: number
  assertionsPassed: number
  assertionsTotal: number
  at: number
}

// The run this page is about: the level's own, and nothing else. A run of another level's is not
// this page's answer, and neither is a run the window has since moved away from.
const mine = computed(() => {
  const run = store.lastRun
  if (!run || run.collectionId !== levelId.value) return null
  return run
})

// What the last run said about each request, by the node it came from.
const answers = computed<Map<string, Answer>>(() => {
  const run = mine.value
  const out = new Map<string, Answer>()
  if (!run) return out
  for (const result of run.results ?? []) {
    out.set(result.nodeId, {
      status: result.status ?? null,
      ok: result.ok,
      skipped: result.skipped ?? false,
      error: result.error ?? '',
      ms: result.durationUs ?? 0,
      assertionsPassed: result.assertionsPassed ?? 0,
      assertionsTotal: result.assertionsTotal ?? 0,
      at: run.finishedAt || run.startedAt,
    })
  }
  return out
})

function answerOf(row: LevelRow): Answer | undefined {
  return answers.value.get(row.id)
}

// What the assertions of one row come to: a request nothing was written about says so rather than
// claiming nothing failed.
function assertsOf(answer: Answer | undefined): string {
  if (!answer || answer.assertionsTotal === 0) return '—'
  return t('collections.ofTotal', { passed: answer.assertionsPassed, total: answer.assertionsTotal })
}

// A row failed when its answer did not come back, or when a script said the answer was wrong. The
// run's own `ok` is about the transport — a 409 is an answer like any other — and a report that is
// worth reading is about what the assertions found as well.
function failed(answer: Answer | undefined): boolean {
  if (!answer || answer.skipped) return false
  return !answer.ok || (answer.assertionsTotal > 0 && answer.assertionsPassed < answer.assertionsTotal)
}

// A row's status says which of the four things happened to it: the answer, a failure, nothing at
// all, or a request a script kept back. The run counts the last as neither, so it is a state of its
// own here too.
function statusOf(answer: Answer | undefined): { text: string; cls: string } {
  if (!answer) return { text: t('collections.notRun'), cls: 'none' }
  if (answer.skipped) return { text: t('collections.skippedShort'), cls: 'none' }
  if (failed(answer)) return { text: t('collections.failed'), cls: 'bad' }
  return { text: String(answer.status), cls: statusBadgeClass(answer.status ?? 0) }
}

// A duration of zero is a row nothing was measured for — a request that never went out — rather than
// an instant one, which is what the window's clock would otherwise let it read as.
function timeOf(answer: Answer | undefined): string {
  if (!answer || answer.ms <= 0) return '—'
  return formatMicros(answer.ms)
}

// The table's rows: the requests of the level and what the run said about each of them, which are
// two lists that meet by node id.
const tableRows = computed(() =>
  requests.value.map((row) => ({ row, answer: answers.value.get(row.id) }))
)

// What the run said about each request, by the node it came from — the row the click hands over.
const results = computed(
  () => new Map((mine.value?.results ?? []).map((result) => [result.nodeId, result]))
)

// A row of the run opens the request it names, with the answer that run got: the row is about that
// request, and what it says is only readable beside the answer it describes. A request the run never
// reached — one that is new since it — opens as the plain card it is.
async function openRow(row: LevelRow) {
  const result = results.value.get(row.id)
  if (result) {
    await store.openRunResult(result)
    return
  }
  await store.select(row.id)
}

// ---- what the level is ----------------------------------------------------

// The folders the level holds — the collections in it, and not the rows it draws. A level's children
// are its requests and its collections merged by position, so counting the lot of them as folders
// said «1 папка» about a folder whose one row is a request.
const folders = computed(() => (level.value?.children ?? []).length)

const meta = computed(() => {
  const parts: string[] = []
  if (trail.value?.collection) {
    // A folder is a collection with a parent, and which collection that is says where it lives.
    const above = trail.value.ancestors[trail.value.ancestors.length - 1]
    if (above) parts.push(t('collections.folderIn', { name: above.name }))
  } else {
    parts.push(t('collections.aCollection'))
  }
  parts.push(t('counts.requests', store.selectedRequestCount))
  if (folders.value > 0) parts.push(t('counts.folders', folders.value))
  return parts.join(' · ')
})

const scheme = computed(() => {
  const auth = trail.value?.collection?.auth
  return auth ? schemeOf(auth.type) : null
})

// The four numbers a run comes to. Every one of them is about the last run rather than about the
// level: what a collection is worth is what it did when it went out.
const stats = computed(() => {
  const run = mine.value
  const rows = run?.results ?? []
  const total = rows.length
  const passed = rows.filter((result) => result.ok).length
  const assertsPassed = rows.reduce((sum, result) => sum + (result.assertionsPassed ?? 0), 0)
  const assertsTotal = rows.reduce((sum, result) => sum + (result.assertionsTotal ?? 0), 0)
  const broken = rows.filter((result) => failed(answers.value.get(result.nodeId)))
  const slowest = rows.reduce<CollectionRunResult | null>(
    (best, result) => (best && best.durationUs >= result.durationUs ? best : result),
    null
  )

  return [
    {
      label: t('collections.statLastRun'),
      value: run ? formatAgo(run.finishedAt || run.startedAt) : '—',
      // The environment the run was made under, not the one on screen now: a run is a thing that
      // happened, and switching environments afterwards does not move it to the other one. A run
      // that kept no environment — one made before runs did — says nothing about it rather than
      // claiming the one that happens to be selected.
      note: run ? lastRunNote(run) : t('collections.notRunYet'),
    },
    {
      label: t('collections.statPassed'),
      value: run ? t('collections.ofTotal', { passed: total - broken.length, total }) : '—',
      note: t('collections.statPassedNote'),
    },
    {
      label: t('collections.statAssertions'),
      value: assertsTotal > 0 ? t('collections.ofTotal', { passed: assertsPassed, total: assertsTotal }) : '—',
      note:
        broken.length > 0
          ? t('collections.statAssertionsFailed', {
              n: broken.length,
              names: broken.map((result) => nameOf(result.nodeId)).join(', '),
            })
          : t('collections.statAssertionsClean'),
    },
    {
      label: t('collections.statSlowest'),
      value: slowest ? formatMicros(slowest.durationUs) : '—',
      note: slowest ? nameOf(slowest.nodeId) : t('collections.notRunYet'),
    },
  ]
})

// The name of a row, for the cards that count rows: the run carries node ids and the page carries the
// names, and the two meet here. A row that came from a folder is named with it — two folders can hold
// a request each under the same name, and a card that said only the name would point at both.
//
// A run walked a request the table no longer holds — one deleted since — and there is nothing left to
// name it by: the id is what is known, and a dash says so rather than inventing one.
function nameOf(nodeId: string): string {
  const row = requests.value.find((it) => it.id === nodeId)
  if (!row) return '—'
  return row.folder ? `${row.folder} · ${row.name}` : row.name
}

// What the run says about itself under the heading: when it went out, and under what.
function lastRunNote(run: CollectionRun): string {
  const total = formatMicros(run.durationUs)
  if (!run.environment) return t('collections.statLastRunNoteBare', { total })
  return t('collections.statLastRunNote', { environment: run.environment, total })
}

// "run 2 min ago · 4 requests" under the heading: what tells a reader whether the rows below are
// about the run they have in mind.
const runNote = computed(() => {
  const run = mine.value
  if (!run) return t('collections.runNever')
  return t('collections.runNote', {
    ago: formatAgo(run.finishedAt || run.startedAt),
    n: (run.results ?? []).length,
  })
})

// A failing row's line under its name: the reason, and how long it took when it was measured.
function reportNote(result: CollectionRunResult): string {
  const parts: string[] = []
  // A refusal the app is behind is said in the window's own words — the code's sentence, in the
  // language the window is in. Only what the machine failed at is repeated as the machine wrote it.
  const refused = refusalText(result.failure)
  if (refused) parts.push(refused)
  else if (result.error) parts.push(result.error)
  else if ((result.assertionsTotal ?? 0) > 0) {
    parts.push(
      t('collections.ofTotal', {
        passed: result.assertionsPassed ?? 0,
        total: result.assertionsTotal ?? 0,
      })
    )
  }
  const time = timeOf(answers.value.get(result.nodeId))
  if (time !== '—') parts.push(time)
  return parts.join(' · ')
}

interface ReportRow {
  label: string
  note: string
  tag: string
  tone: 'bad' | 'ok' | 'flat'
  // The request the line is about, where there is one: a line that names a request is a way into it,
  // and the summary line — how many passed — names nobody.
  result?: CollectionRunResult
}

// The run, said in a few lines: what failed, what that leaves, and what took longest. It is the same
// run the table above draws, and it is worth a panel of its own because a failure is a thing to read
// rather than a row to scan past.
const report = computed<ReportRow[]>(() => {
  const run = mine.value
  if (!run) return []
  const rows = run.results ?? []
  const broken = rows.filter((result) => failed(answers.value.get(result.nodeId)))
  const passed = rows.filter((result) => !failed(answers.value.get(result.nodeId)))
  const assertsPassed = rows.reduce((sum, result) => sum + (result.assertionsPassed ?? 0), 0)
  const assertsTotal = rows.reduce((sum, result) => sum + (result.assertionsTotal ?? 0), 0)
  const slowest = rows.reduce<CollectionRunResult | null>(
    (best, result) => (best && best.durationUs >= result.durationUs ? best : result),
    null
  )

  const out: ReportRow[] = broken.map((result) => ({
    label: t('collections.reportFailedRow', {
      name: nameOf(result.nodeId),
      status: result.status ? String(result.status) : t('collections.failed'),
    }),
    // What went wrong, said with what the row has: a request that never came back has an error to
    // read and nothing asserted, and a request that came back wrong has the assertions instead.
    note: reportNote(result),
    tag: t('collections.failed'),
    tone: 'bad',
    result,
  }))
  out.push({
    label: t('collections.reportPassedRow', { n: passed.length }),
    note: t('collections.reportPassedNote', {
      asserts: t('collections.assertsCount', { passed: assertsPassed, total: assertsTotal }),
      total: formatMicros(run.durationUs),
    }),
    tag: t('collections.passed'),
    tone: 'ok',
  })
  if (slowest) {
    const row = requests.value.find((it) => it.id === slowest.nodeId)
    out.push({
      label: t('collections.reportSlowestRow', { name: nameOf(slowest.nodeId) }),
      note: t('collections.reportSlowestNote', {
        ms: timeOf(answers.value.get(slowest.nodeId)),
        method: row?.method ?? '—',
        path: addressOf(row?.url ?? ''),
      }),
      tag: formatMicros(slowest.durationUs),
      tone: 'flat',
      result: slowest,
    })
  }
  return out
})

// ---- the three things the level owns --------------------------------------

type Sheet = 'auth' | 'scripts' | 'variables' | 'report'

const sheet = ref<Sheet | null>(null)

const sheetTitle = computed(() => (sheet.value ? t(`collections.${sheet.value}Sheet`) : ''))

const variablesNote = computed(() => {
  const own = trail.value?.collection?.variables ?? []
  if (own.length === 0) return t('collections.variablesNone')
  return t('counts.variables', own.length)
})

const scriptsNote = computed(() => {
  const own = store.scripts
  if (!own || (!own.pre && !own.post)) return t('collections.scriptsNone')
  return t('collections.scriptsSome')
})

const authNote = computed(() => {
  if (scheme.value?.note) return t(scheme.value.note)
  return scheme.value ? t('collections.authInheritedByChildren') : t('collections.authNoneBody')
})

// The footer's pair is the sheet's own: it closes either way, and what the editor was holding is
// written or dropped. An editor that wrote while the user typed would leave Cancel nothing to do.
type Editor = { commit: () => Promise<void>; discard: () => void } | null

const authEditor = ref<Editor>(null)
const variablesEditor = ref<Editor>(null)
const scriptsEditor = ref<Editor>(null)

const editor = computed<Editor>(() => {
  switch (sheet.value) {
    case 'auth':
      return authEditor.value
    case 'variables':
      return variablesEditor.value
    case 'scripts':
      return scriptsEditor.value
    default:
      return null
  }
})

async function saveSheet() {
  await editor.value?.commit()
  sheet.value = null
}

function closeSheet() {
  editor.value?.discard()
  sheet.value = null
}

// The report's own line: which run a reader is looking at. A sheet opens from a card and the card's
// own sentence is what it says about itself; this one is opened from the run, so its line is the
// run's.
const reportSub = computed(() => {
  const run = mine.value
  if (!run) return ''
  const facts = {
    name: title.value,
    n: (run.results ?? []).length,
    ago: formatAgo(run.finishedAt || run.startedAt),
  }
  // The run's own environment, for the same reason the stat above is drawn with it.
  if (!run.environment) return t('collections.reportSubBare', facts)
  return t('collections.reportSub', { ...facts, environment: run.environment })
})

// What a sheet is about, in a line under its name — the drawing gives every one of them a sentence,
// and the card it was opened from is where the sentence comes from.
const sheetSub = computed(() => {
  switch (sheet.value) {
    case 'auth':
      return authNote.value
    case 'variables':
      return variablesNote.value
    case 'scripts':
      return scriptsNote.value
    case 'report':
      return reportSub.value
    default:
      return ''
  }
})

async function run() {
  await store.run(store.runNodeId, title.value)
}

</script>

<template>
  <div class="overview">
    <header class="head">
      <span class="folder"><Icon name="folder" :size="23" :stroke-width="1.7" /></span>
      <div class="titles">
        <h1 class="title">{{ title }}</h1>
        <span class="meta">{{ meta }}</span>
        <input
          v-if="editingDescription"
          ref="descriptionInput"
          v-model="descriptionDraft"
          class="description-input"
          spellcheck="false"
          @blur="commitDescription"
          @keydown="onDescriptionKeydown"
        />
        <span v-else class="description" :class="{ placeholder: !description }" @click="editDescription">
          {{ description || t('collections.addDescription') }}
        </span>
      </div>
      <Button
        class="run-folder"
        variant="primary"
        size="page"
        :disabled="store.selectedRequestCount === 0"
        @click="run"
      >
        <Icon :name="store.running ? 'stop' : 'play'" :size="15" />
        {{ store.running ? t('collections.stop') : t('collections.runFolder') }}
      </Button>
      <Button size="page" :disabled="store.running" @click="importCollection">
        <Icon name="download" :size="15" />
        {{ t('collections.import') }}
      </Button>
      <Button size="page" :disabled="store.running" @click="exportLevel">
        <Icon name="upload" :size="15" />
        {{ t('collections.export') }}
      </Button>
    </header>

    <div class="stats">
      <div v-for="stat in stats" :key="stat.label" class="stat">
        <span class="stat-label">{{ stat.label }}</span>
        <span class="stat-value">{{ stat.value }}</span>
        <span class="stat-note">{{ stat.note }}</span>
      </div>
    </div>

    <section class="requests">
      <div class="section-head">
        <span class="section-label">{{ t('collections.runResults') }}</span>
        <span class="section-note">{{ runNote }}</span>
        <span class="head-spacer"></span>
        <button v-if="mine" type="button" class="report-link" @click="sheet = 'report'">
          {{ t('collections.openReport') }}
        </button>
      </div>
      <div class="table">
        <div class="table-head">
          <span>{{ t('collections.columnMethod') }}</span>
          <span>{{ t('collections.columnName') }}</span>
          <span>{{ t('collections.columnAssertions') }}</span>
          <span>{{ t('collections.columnTime') }}</span>
          <span>{{ t('collections.columnStatus') }}</span>
        </div>
        <div class="table-body">
          <button
            v-for="entry in tableRows"
            :key="entry.row.id"
            type="button"
            class="table-row"
            @click="openRow(entry.row)"
          >
            <span class="cell-method mono" :class="methodInkClass(entry.row.method ?? '')">
              {{ entry.row.method }}
            </span>
            <!-- The folder a row came from, where it came from one: the table holds the whole
                 collection, and two folders can both call a request "Список". -->
            <span class="cell-name">
              <span v-if="entry.row.folder" class="cell-folder">{{ entry.row.folder }}</span>
              {{ entry.row.name }}
            </span>
            <span class="cell-asserts mono" :class="{ bad: failed(entry.answer) }">
              {{ assertsOf(entry.answer) }}
            </span>
            <span class="cell-time mono">{{ timeOf(entry.answer) }}</span>
            <span class="cell-status">
              <span class="status-pill" :class="statusOf(entry.answer).cls">
                {{ statusOf(entry.answer).text }}
              </span>
            </span>
          </button>
          <div v-if="!loading && tableRows.length === 0" class="table-empty">
            {{ t('collections.noRequestsHere') }}
          </div>
        </div>
      </div>
    </section>

    <div class="cards">
      <div class="card">
        <span class="card-title">{{ t('collections.authSheet') }}</span>
        <span class="card-body">{{ authNote }}</span>
        <button type="button" class="card-action" @click="sheet = 'auth'">
          {{ t('collections.editAuth') }}
        </button>
      </div>
      <div class="card">
        <span class="card-title">{{ t('collections.variablesSheet') }}</span>
        <span class="card-body">{{ variablesNote }}</span>
        <button type="button" class="card-action" @click="sheet = 'variables'">
          {{ t('collections.editVariables') }}
        </button>
      </div>
      <div class="card">
        <span class="card-title">{{ t('collections.scriptsSheet') }}</span>
        <span class="card-body">{{ scriptsNote }}</span>
        <button type="button" class="card-action" @click="sheet = 'scripts'">
          {{ t('collections.editScripts') }}
        </button>
      </div>
    </div>

    <!-- The editors are leaves over the page rather than tabs inside it: a collection's authorization
         is set once and left, and a tab that is nearly always closed is a line of the page spent on
         nothing. -->
    <!-- The report is read, not edited: it has nothing to cancel, and a Cancel beside its Close would
         be two names for the one thing that button does. -->
    <Sheet
      :open="sheet !== null"
      :title="sheetTitle"
      :sub="sheetSub"
      :cancel="sheet !== null && sheet !== 'report' ? t('common.cancel') : ''"
      :action="sheet === 'report' ? t('common.close') : t('common.save')"
      @close="closeSheet"
      @cancel="closeSheet"
      @action="sheet === 'report' ? closeSheet() : saveSheet()"
    >
      <CollectionAuth v-if="sheet === 'auth'" ref="authEditor" />
      <CollectionVariables v-else-if="sheet === 'variables'" ref="variablesEditor" />
      <CollectionScripts v-else-if="sheet === 'scripts'" ref="scriptsEditor" />
      <div v-else class="report">
        <button
          v-for="(line, i) in report"
          :key="i"
          type="button"
          class="report-row"
          :class="{ linked: line.result }"
          :disabled="!line.result"
          @click="line.result && store.openRunResult(line.result)"
        >
          <span class="report-text">
            <span class="report-label">{{ line.label }}</span>
            <span class="report-note">{{ line.note }}</span>
          </span>
          <span class="report-tag" :class="line.tone">{{ line.tag }}</span>
        </button>
      </div>
    </Sheet>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.overview {
  /* The handoff is plain HTML, so its every line is drawn at `normal`. The window's base is Tailwind's
     1.5, which leaves each label, value and title on this page a couple of pixels looser than the
     drawing — the page's own blocks are the ones it shows on. */
  line-height: normal;
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-6.5 bg-bg-panel;
  padding: 28px 32px;
}

/* «Запустить папку» is the handoff's own one-off button: a shade larger than the two beside it, which
   are Button.vue's plain page size. It is written here rather than added to that file because the
   drawing gives no other button these numbers. */
.head .btn.run-folder {
  gap: 9px;
  padding: 0 16px;
  font-size: 14px;
  font-weight: 600;
}

.head {
  @apply flex items-start gap-4;
}

.folder {
  @apply flex-none inline-flex items-center justify-center w-11 h-11 rounded-xl
         bg-accent-soft text-accent;
}

.titles {
  @apply flex flex-col flex-1 min-w-0 gap-1.5;
}

.title {
  @apply m-0 text-[22px] font-semibold;
  letter-spacing: -0.01em;
  @apply overflow-hidden text-ellipsis whitespace-nowrap;
}

.meta {
  @apply text-[13.5px] text-text-secondary;
}

.description {
  @apply text-[13px] text-text-secondary cursor-text;
  @apply overflow-hidden text-ellipsis whitespace-nowrap;
}

/* Not `.empty`: that name is the window's own empty state — a centred column with room around a glyph
   — and a description nobody has written yet is a line of text like any other, left under the title. */
.description.placeholder {
  @apply text-text-tertiary;
}

.description-input {
  @apply w-full bg-transparent border-0 outline-none p-0 text-[13px] text-text;
}

/* Four counters on one line: a collection's whole state is worth a glance, and a glance is one line. */
.stats {
  @apply grid grid-cols-4 gap-3;
}

.stat {
  @apply flex flex-col gap-1.5 border border-border rounded-xl;
  padding: 14px 16px;
}

.stat-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.stat-value {
  @apply text-[20px] font-semibold;
  letter-spacing: -0.01em;
}

.stat-note {
  @apply text-[12.5px] text-text-tertiary;
}

.requests {
  @apply flex flex-col gap-2.5;
}

/* The heading of the run's own block: what it is, when it last happened, and the way into the full
   report. */
.section-head {
  @apply flex items-baseline gap-2.5;
}

.section-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.section-note {
  @apply text-[12.5px] text-text-tertiary;
}

.report-link {
  @apply h-7 px-2 -my-1 rounded-[7px] border-0 bg-transparent cursor-pointer
         text-[12.5px] font-medium text-accent;
  font-family: inherit;
}

.report-link:hover {
  @apply bg-accent-soft;
}

/* The level's rows, capped at the height the handoff draws: a collection of fifty requests is a list
   to scroll inside a page rather than a page of its own. */
.table {
  @apply flex flex-col border border-border rounded-xl overflow-hidden max-h-[320px];
}

.table-body {
  @apply flex-1 min-h-0 overflow-y-auto;
}

.table-head,
.table-row {
  @apply grid items-center;
  grid-template-columns: 90px minmax(0, 1fr) 200px 120px 110px;
}

.table-head {
  @apply bg-bg-inset border-b border-border text-[11px] font-semibold uppercase
         tracking-[0.06em] text-text-tertiary;
}

.table-head > span {
  padding: 10px 16px;
}

.table-row {
  @apply w-full text-left border-0 border-b border-border bg-transparent cursor-pointer
         min-h-[46px] text-text;
}

.table-row:hover {
  background: var(--bg-hover);
}

.table-row:last-child {
  border-bottom: 0;
}

.table-row > span {
  @apply min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
  padding: 0 16px;
}

.cell-method {
  @apply text-[11.5px];
}

.cell-name {
  @apply text-[13.5px] overflow-hidden text-ellipsis whitespace-nowrap;
}

/* The folder is where the row came from and not what it is called: it stands before the name in the
   window's third shade, so a glance down the column still reads the requests. */
.cell-folder {
  @apply text-text-tertiary;
}

.cell-folder::after {
  content: ' · ';
}

.cell-asserts {
  @apply text-[12.5px] text-text-secondary;
}

.cell-asserts.bad {
  color: var(--red-text);
}

.cell-time {
  @apply text-[12.5px] text-text-secondary;
}

.status-pill {
  @apply inline-flex items-center text-[11.5px] font-semibold py-[3px] px-2 rounded-md;
}

.status-pill.none {
  @apply text-text-tertiary;
  background: var(--bg-hover);
}

.status-pill.bad {
  color: var(--red-text);
  background: var(--red-soft);
}

.table-empty {
  @apply text-[13px] text-text-tertiary;
  padding: 14px 16px;
}

.cards {
  @apply grid grid-cols-3 gap-3;
}

.card {
  @apply flex flex-col items-start gap-2 border border-border rounded-xl;
  padding: 16px;
}

.card-title {
  @apply text-[13.5px] font-semibold;
}

.card-body {
  @apply text-[13px] leading-normal text-text-secondary;
}

/* The three cards stand in a row and their sentences are of different lengths, so the action is
   pushed to the foot of its card: three buttons at three heights read as three different things. */
.card-action {
  @apply self-start mt-auto h-[30px] px-2 -ml-2 border-0 rounded-[7px] bg-transparent cursor-pointer
         text-[13px] font-medium text-accent;
  font-family: inherit;
}

.card-action:hover {
  @apply bg-accent-soft;
}

/* The sheet is a leaf of its own, sized to one editor: the environments' sheet is a whole workspace
   with a list beside the table, and a collection's authorization is a form. */

/* The run's report: the failures one by one, then what the run came to, then the slowest row — the
   three things worth reading about a run that has already been drawn row by row above. */
.report {
  @apply flex flex-col gap-0.5;
}

/* A line of the report is a button where it names a request — a failure and the slowest one are both
   ways into what they are about — and a plain line where it does not. */
.report-row {
  @apply flex items-center w-full gap-3 min-h-11 px-2.5 rounded-[9px] border-0 bg-transparent
         text-left text-text;
  font-family: inherit;
}

.report-row.linked {
  @apply cursor-pointer;
}

.report-row.linked:hover {
  @apply bg-bg-hover;
}

.report-text {
  @apply flex-1 min-w-0 flex flex-col gap-0.5;
}

.report-label {
  @apply text-[13.5px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.report-note {
  @apply text-[12px] text-text-tertiary;
}

.report-tag {
  @apply flex-none text-[12.5px] font-semibold py-1 px-[9px] rounded-md text-text-tertiary;
  background: var(--bg-hover);
}

.report-tag.bad {
  color: var(--red-text);
  background: var(--red-soft);
}

.report-tag.ok {
  @apply bg-green-soft;
  color: var(--green-text);
}
</style>
