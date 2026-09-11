<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, type ComponentPublicInstance } from 'vue'
import Icon from './Icon.vue'
import {
  parseDotenv,
  useEnvironmentsStore,
  type ImportChoice,
  type Variable,
} from '../stores/environments'
import { useRequestsStore } from '../stores/requests'
import { parseTokens } from '../lib/vars'
import { shortcut } from '../lib/platform'

const envStore = useEnvironmentsStore()
const reqStore = useRequestsStore()

const emit = defineEmits<{ (e: 'close'): void }>()

const editHint = shortcut('E')

// Which environment the table on the right is editing. `null` is "Глобальные",
// which is a first-class scope rather than a separate screen.
const envId = computed(() => envStore.sheetEnvId)
const env = computed(() => envStore.environments.find((e) => e.id === envId.value) ?? null)
const isGlobals = computed(() => envId.value === null)

// Read-only is a property of the environment, lifted for the session by an
// explicit unlock (see closeSheet — the lift never outlives the sheet).
const locked = computed(() => Boolean(env.value?.readonly) && !envStore.unlocked.includes(envId.value as string))

const vars = computed(() => envStore.varsOf(envId.value))

const filter = ref('')
const filteredVars = computed(() => {
  const q = filter.value.trim().toLowerCase()
  if (!q) return vars.value
  return vars.value.filter((v) => v.name.toLowerCase().includes(q))
})

const showSecrets = ref(false)

function displayValue(v: Variable): string {
  if (v.kind !== 'secret') return v.value
  return showSecrets.value ? envStore.varValue(envId.value, v) : '••••'
}

// --- Inline editing ---------------------------------------------------------
// One cell at a time, so a single draft ref is enough. Enter commits, Esc
// discards, Tab commits and moves on — the table behaves like a spreadsheet.
interface Editing {
  varId: string
  field: 'name' | 'value'
}

const editing = ref<Editing | null>(null)
const draft = ref('')
const error = ref('')
const cellInput = ref<HTMLInputElement | null>(null)

// A function ref, not `ref="cellInput"`: refs inside a v-for are collected into
// an array, and only one cell is ever in edit mode anyway.
function setCellInput(el: Element | ComponentPublicInstance | null) {
  cellInput.value = (el as HTMLInputElement | null) ?? null
}

function startEdit(v: Variable, field: 'name' | 'value') {
  if (locked.value) return
  editing.value = { varId: v.id, field }
  error.value = ''
  // Editing a secret starts from its real value, so committing doesn't wipe it;
  // the alternative (editing a placeholder) would destroy the stored secret on
  // the first keystroke.
  draft.value = field === 'name' ? v.name : envStore.varValue(envId.value, v)
  nextTick(() => cellInput.value?.focus())
}

// Blur fires on the cell being left even when Tab has already opened the next
// one, so a blind commit on blur would validate and close the cell the user
// just moved into.
function commitFrom(v: Variable, field: 'name' | 'value') {
  const ed = editing.value
  if (!ed || ed.varId !== v.id || ed.field !== field) return
  commit()
}

function commit(): boolean {
  const ed = editing.value
  if (!ed) return false
  const v = vars.value.find((x) => x.id === ed.varId)
  if (!v) {
    editing.value = null
    return false
  }
  if (ed.field === 'name') {
    const name = draft.value.trim()
    if (!name) {
      error.value = 'Имя не может быть пустым'
      return false
    }
    if (vars.value.some((x) => x.id !== v.id && x.name === name)) {
      error.value = 'Такое имя уже есть в этом окружении'
      return false
    }
    envStore.updateVar(envId.value, v.id, { name })
  } else {
    envStore.updateVar(envId.value, v.id, { value: draft.value })
  }
  editing.value = null
  error.value = ''
  return true
}

function cancel() {
  editing.value = null
  error.value = ''
}

// Tab walks name → value → next row's name, so a whole environment can be
// filled without touching the mouse.
function moveTo(v: Variable, field: 'name' | 'value') {
  const at = filteredVars.value.indexOf(v)
  const next = filteredVars.value[at + 1]
  if (field === 'value' && next) startEdit(next, 'name')
  else if (field === 'value') cancel()
  else startEdit(v, 'value')
}

function onCellKeydown(e: KeyboardEvent, v: Variable, field: 'name' | 'value') {
  if (e.key === 'Enter') {
    e.preventDefault()
    if (commit()) if (field === 'name') startEdit(v, 'value')
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  } else if (e.key === 'Tab') {
    e.preventDefault()
    if (commit()) moveTo(v, field)
  }
}

// --- Structure edits --------------------------------------------------------

function addVar() {
  if (locked.value) return
  const id = envStore.addVar(envId.value, { name: '', value: '' })
  const created = vars.value.find((v) => v.id === id)
  if (created) startEdit(created, 'name')
}

function removeVar(v: Variable) {
  if (locked.value) return
  envStore.removeVar(envId.value, v.id)
}

function toggleKind(v: Variable) {
  if (locked.value) return
  envStore.updateVar(envId.value, v.id, { kind: v.kind === 'secret' ? 'text' : 'secret' })
}

function addEnv() {
  const id = envStore.addEnv()
  envStore.selectSheetEnv(id)
}

// --- Removing an environment ------------------------------------------------

const confirming = ref<string | null>(null)

// A variable that no saved request mentions is safe to drop silently; one that
// is referenced everywhere deserves a question first.
function referencedBy(name: string): boolean {
  const texts: string[] = [reqStore.draft.url, reqStore.draft.body]
  for (const p of reqStore.draft.params) texts.push(p.name, p.value)
  for (const h of reqStore.draft.headers) texts.push(h.name, h.value)
  for (const r of reqStore.requests) {
    texts.push(r.url, r.requestBody)
    for (const [k, v] of Object.entries(r.requestHeaders ?? {})) texts.push(k, v)
  }
  return texts.some((t) => t && parseTokens(t).some((tok) => tok.name === name))
}

function askRemove(id: string) {
  const target = envStore.environments.find((e) => e.id === id)
  if (!target) return
  if (target.vars.some((v) => referencedBy(v.name))) {
    confirming.value = id
    return
  }
  doRemove(id)
}

function doRemove(id: string) {
  envStore.removeEnv(id)
  confirming.value = null
  // Deleting what you were looking at leaves the sheet with nothing to show.
  if (envStore.sheetEnvId === id) envStore.selectSheetEnv(envStore.environments[0]?.id ?? null)
}

// --- Import .env ------------------------------------------------------------
//
// The file is read in the webview (a file input) rather than through a Go
// binding — nothing about parsing a text file needs the backend.
const fileInput = ref<HTMLInputElement | null>(null)
const importEntries = ref<ImportChoice[] | null>(null)

function pickFile() {
  if (isGlobals.value) return
  fileInput.value?.click()
}

async function onFileChosen(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  // Reset immediately, so choosing the same file twice still fires a change.
  input.value = ''
  if (!file) return
  const text = await file.text()
  // Conflicts default to "skip": an import must never overwrite a value the
  // user set by hand without saying so on the row.
  importEntries.value = parseDotenv(text).map((entry) => ({
    ...entry,
    mode: existingNames.value.has(entry.name) ? ('skip' as const) : ('replace' as const),
  }))
}

const existingNames = computed(() => new Set(vars.value.map((v) => v.name)))

const importCount = computed(
  () => importEntries.value?.filter((e) => e.mode === 'replace' || !existingNames.value.has(e.name)).length ?? 0
)

function applyImport() {
  if (importEntries.value) envStore.importDotenv(envId.value, importEntries.value)
  importEntries.value = null
}

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  // Esc backs out one level at a time: the open cell, then whatever dialog is
  // on top, then the sheet itself.
  if (editing.value) {
    cancel()
    return
  }
  if (importEntries.value) {
    importEntries.value = null
    return
  }
  if (confirming.value) {
    confirming.value = null
    return
  }
  emit('close')
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="sheet-overlay">
    <div class="sheet">
      <div class="sheet-head">
        <span class="sheet-title">Переменные окружения</span>
        <span class="sheet-hint mono">{{ editHint }}</span>
        <button class="btn sheet-done" @click="emit('close')">Готово</button>
      </div>

      <div class="sheet-body">
        <!-- Left: environments + globals -->
        <div class="sheet-side">
          <div class="side-label">Окружения</div>
          <button
            v-for="e in envStore.environments"
            :key="e.id"
            class="side-row"
            :class="{ active: e.id === envStore.sheetEnvId }"
            @click="envStore.selectSheetEnv(e.id)"
          >
            <span class="side-dot" :class="{ on: e.id === envStore.activeId }"></span>
            <span class="side-name">{{ e.name }}</span>
            <Icon v-if="e.readonly" name="lock" :size="11" class="side-lock" />
            <span v-else class="side-count mono">{{ e.vars.length }}</span>
          </button>

          <div class="side-divider"></div>

          <button
            class="side-row"
            :class="{ active: envStore.sheetEnvId === null }"
            @click="envStore.selectSheetEnv(null)"
          >
            <span class="side-dot"></span>
            <span class="side-name">Глобальные</span>
            <span class="side-count mono">{{ envStore.globals.length }}</span>
          </button>

          <div class="side-spacer"></div>

          <div class="side-foot">
            <button class="side-icon-btn" title="Новое окружение" @click="addEnv">
              <Icon name="plus" :size="14" />
            </button>
            <button
              class="side-icon-btn"
              title="Удалить окружение"
              :disabled="isGlobals"
              @click="isGlobals || askRemove(envStore.sheetEnvId as string)"
            >
              <Icon name="minus" :size="14" />
            </button>
            <span class="side-foot-spacer"></span>
            <input
              ref="fileInput"
              type="file"
              accept=".env,text/plain"
              class="file-input"
              @change="onFileChosen"
            />
            <button
              class="side-text-btn"
              :disabled="isGlobals"
              :title="isGlobals ? 'Импорт идёт в выбранное окружение, не в глобальные' : undefined"
              @click="pickFile"
            >
              Импорт .env
            </button>
          </div>
        </div>

        <!-- Right: the table -->
        <div class="sheet-main">
          <div class="sheet-toolbar">
            <div class="filter">
              <Icon name="search" :size="12" />
              <input v-model="filter" class="filter-input" placeholder="Фильтр по имени" spellcheck="false" />
            </div>
            <span class="toolbar-spacer"></span>
            <template v-if="locked">
              <span class="toolbar-note">Окружение только для чтения</span>
              <button class="btn tile-btn" @click="envStore.unlock(envStore.sheetEnvId as string)">Разблокировать</button>
            </template>
            <template v-else>
              <span class="toolbar-note">{{ showSecrets ? 'Значения секретов показаны' : 'Значения секретов скрыты' }}</span>
              <button class="btn tile-btn" @click="showSecrets = !showSecrets">
                {{ showSecrets ? 'Скрыть' : 'Показать' }}
              </button>
            </template>
          </div>

          <div class="table-head">
            <div>Переменная</div>
            <div>Значение</div>
            <div>Тип</div>
            <div></div>
          </div>

          <div class="table-body">
            <div v-for="v in filteredVars" :key="v.id" class="row">
              <!-- name -->
              <div class="cell cell-name">
                <input
                  v-if="editing && editing.varId === v.id && editing.field === 'name'"
                  :ref="setCellInput"
                  v-model="draft"
                  class="cell-input mono"
                  :class="{ invalid: error }"
                  spellcheck="false"
                  @keydown="onCellKeydown($event, v, 'name')"
                  @blur="commitFrom(v, 'name')"
                />
                <span v-else class="cell-text mono" :title="v.name" @click="startEdit(v, 'name')">{{ v.name }}</span>
              </div>

              <!-- value -->
              <div class="cell cell-value">
                <input
                  v-if="editing && editing.varId === v.id && editing.field === 'value'"
                  :ref="setCellInput"
                  v-model="draft"
                  class="cell-input mono"
                  spellcheck="false"
                  @keydown="onCellKeydown($event, v, 'value')"
                  @blur="commitFrom(v, 'value')"
                />
                <span
                  v-else
                  class="cell-text mono"
                  :class="{ masked: v.kind === 'secret' && !showSecrets }"
                  @click="startEdit(v, 'value')"
                  >{{ displayValue(v) }}</span
                >
              </div>

              <!-- type -->
              <div class="cell">
                <span v-if="isGlobals" class="tag tag-global">глобальная</span>
                <button
                  v-else
                  class="tag"
                  :class="v.kind === 'secret' ? 'tag-secret' : 'tag-text'"
                  :disabled="locked"
                  :title="locked ? 'Окружение только для чтения' : 'Переключить тип'"
                  @click="toggleKind(v)"
                >
                  {{ v.kind === 'secret' ? 'секрет' : 'текст' }}
                </button>
              </div>

              <div class="cell cell-action">
                <button v-if="!locked" class="row-del" title="Удалить" @click="removeVar(v)">
                  <Icon name="trash" :size="13" />
                </button>
              </div>
            </div>

            <div v-if="filteredVars.length === 0" class="table-empty">
              {{ vars.length === 0 ? 'Переменных пока нет' : 'Ничего не найдено' }}
            </div>

            <button class="new-row" :disabled="locked" @click="addVar">
              <Icon name="plus" :size="13" />
              <span>Новая переменная</span>
            </button>
          </div>

          <div class="sheet-foot">
            <span v-if="error" class="foot-error">{{ error }}</span>
            <span v-else-if="!envStore.keychainAvailable" class="foot-warn">
              Связка ключей недоступна — секреты не сохранятся после выхода
            </span>
            <span v-else>Секреты хранятся в связке ключей macOS и не попадают в экспорт коллекции</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Import confirmation: one review pass, then one button -->
    <div v-if="importEntries" class="confirm-overlay" @click.self="importEntries = null">
      <div class="import">
        <div class="import-title">
          Импорт .env → {{ env?.name ?? 'Глобальные' }}
        </div>
        <div class="import-body">
          <div v-for="e in importEntries" :key="e.name" class="import-row">
            <button
              class="import-secret"
              :class="{ on: e.secret }"
              :title="e.secret ? 'Импортировать как секрет' : 'Импортировать как обычную переменную'"
              @click="e.secret = !e.secret"
            >
              <Icon v-if="e.secret" name="check" :size="10" />
            </button>
            <span class="import-name mono">{{ e.name }}</span>
            <span class="import-value mono" :class="{ masked: e.secret }">{{ e.secret ? '••••' : e.value }}</span>
            <span v-if="existingNames.has(e.name)" class="import-conflict">
              <button class="conflict-btn" :class="{ on: e.mode === 'skip' }" @click="e.mode = 'skip'">пропустить</button>
              <button class="conflict-btn" :class="{ on: e.mode === 'replace' }" @click="e.mode = 'replace'">заменить</button>
            </span>
            <span v-else class="import-new">новая</span>
          </div>
          <div v-if="importEntries.length === 0" class="import-empty">
            В файле не нашлось строк вида KEY=value
          </div>
        </div>
        <div class="import-actions">
          <span class="import-summary">Будет записано: {{ importCount }}</span>
          <button class="btn" @click="importEntries = null">Отмена</button>
          <button class="btn btn-primary" :disabled="importCount === 0" @click="applyImport">Импортировать</button>
        </div>
      </div>
    </div>

    <!-- Deleting an environment that a saved request still points at -->
    <div v-if="confirming" class="confirm-overlay" @click.self="confirming = null">
      <div class="confirm">
        <div class="confirm-title">Удалить окружение?</div>
        <div class="confirm-body">
          На переменные окружения <b>{{ envStore.environments.find((e) => e.id === confirming)?.name }}</b>
          ссылается сохранённый запрос. После удаления его токены станут неизвестными.
        </div>
        <div class="confirm-actions">
          <button class="btn" @click="confirming = null">Отмена</button>
          <button class="btn btn-primary" @click="doRemove(confirming)">Удалить</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.sheet-overlay {
  @apply fixed inset-0 z-1500 flex items-center justify-center;
  background: rgba(0, 0, 0, 0.25);
}

.sheet {
  @apply flex flex-col rounded-xl overflow-hidden w-[1040px] max-w-[95vw];
  background: var(--bg-panel);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.18), 0 0 0 1px var(--border);
}

.sheet-head {
  @apply flex-none flex items-center gap-3 h-[46px] px-3.5 border-b border-border;
  background: var(--bg-sidebar);
}

.sheet-title {
  @apply text-[13px] font-semibold;
}

.sheet-hint {
  @apply flex-1 text-[10.5px] text-text-tertiary;
}

.sheet-done {
  @apply h-[26px] py-0 text-xs;
}

.sheet-body {
  @apply flex h-[420px] min-h-0;
}

/* ---- left column ---- */
.sheet-side {
  @apply flex-none w-[232px] flex flex-col p-2 pb-0 border-r border-border;
  background: var(--bg-inset);
}

.side-label {
  @apply pt-1.5 px-2 pb-1.5 text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
  font-family: var(--mono);
}

.side-row {
  @apply flex items-center gap-2 w-full py-[7px] px-2 border-none rounded-md bg-transparent text-text text-[13px] text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.side-row:hover {
  @apply bg-bg-hover;
}

.side-row.active {
  @apply bg-accent-soft font-semibold;
}

.side-dot {
  @apply flex-none w-[7px] h-[7px] rounded-full;
}

.side-dot.on {
  background: var(--green);
}

.side-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap;
}

.side-count {
  @apply flex-none text-[10.5px] text-text-tertiary;
}

.side-lock {
  @apply flex-none text-text-tertiary;
}

.side-divider {
  @apply h-px mx-1.5 my-2 bg-border;
}

.side-spacer {
  @apply flex-1;
}

.side-foot {
  @apply flex items-center gap-0.5 py-1.5 border-t border-border;
}

.side-foot-spacer {
  @apply flex-1;
}

.side-icon-btn {
  @apply w-[26px] h-[26px] border-none rounded-md bg-transparent text-text-secondary cursor-pointer inline-flex items-center justify-center;
}

.side-icon-btn:hover:not(:disabled) {
  @apply bg-bg-active text-text;
}

.side-icon-btn:disabled {
  @apply opacity-40 cursor-default;
}

.side-text-btn {
  @apply h-[26px] px-2 border-none rounded-md bg-transparent text-text-secondary text-xs cursor-pointer;
  font: inherit;
}

.side-text-btn:hover:not(:disabled) {
  @apply bg-bg-active text-text;
}

.side-text-btn:disabled {
  @apply opacity-50 cursor-default;
}

/* ---- right column ---- */
.sheet-main {
  @apply flex-1 min-w-0 flex flex-col;
}

.sheet-toolbar {
  @apply flex-none flex items-center gap-2.5 h-10 px-3.5 border-b border-border;
}

.filter {
  @apply flex items-center gap-1.5 h-[26px] px-2.5 rounded-md w-full max-w-[260px] text-text-tertiary;
  background: var(--bg-hover);
}

.filter-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none text-xs text-text;
  font: inherit;
}

.toolbar-spacer {
  @apply flex-1;
}

.toolbar-note {
  @apply text-xs text-text-secondary whitespace-nowrap;
}

.tile-btn {
  @apply h-[26px] py-0 text-xs whitespace-nowrap;
}

.table-head,
.row {
  @apply grid items-center gap-0 px-3.5;
  grid-template-columns: 200px minmax(0, 1fr) 96px 34px;
}

.table-head {
  @apply flex-none h-[30px] border-b border-border text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
  font-family: var(--mono);
}

.table-body {
  @apply flex-1 min-h-0 overflow-auto;
}

.row {
  @apply h-[38px] border-b;
  border-color: color-mix(in srgb, var(--border) 60%, transparent);
}

.row:hover {
  background: color-mix(in srgb, var(--text) 2.5%, transparent);
}

.cell {
  @apply min-w-0 flex items-center;
}

.cell-name {
  @apply pr-3;
}

.cell-value {
  @apply pr-3;
}

.cell-text {
  @apply w-full overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px] cursor-text;
}

.cell-text.masked {
  @apply text-text-tertiary;
}

.cell-input {
  @apply w-full min-w-0 text-[12.5px] px-1 py-0.5 rounded-sm outline-none;
  background: var(--bg-inset);
  border: 1px solid var(--accent);
  color: var(--text);
}

.cell-input.invalid {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
}

.tag {
  @apply text-[10px] font-semibold py-px px-[5px] rounded-sm border-none;
}

.tag-text {
  @apply text-text-secondary cursor-pointer;
  background: var(--bg-hover);
}

.tag-secret {
  @apply text-orange bg-orange-soft cursor-pointer;
}

.tag-global {
  @apply text-purple;
  background: color-mix(in srgb, var(--purple) 13%, transparent);
}

.cell-action {
  @apply justify-end;
}

.row-del {
  @apply w-[22px] h-[22px] border-none rounded-md bg-transparent text-text-tertiary cursor-pointer inline-flex items-center justify-center;
}

.row-del:hover {
  @apply text-red bg-red-soft;
}

.table-empty {
  @apply px-3.5 py-3 text-xs text-text-tertiary;
}

.new-row {
  @apply flex items-center gap-1.5 w-full h-[38px] px-3.5 border-none bg-transparent text-text-tertiary text-[12.5px] text-left cursor-pointer;
  font: inherit;
}

.new-row:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}

.new-row:disabled {
  @apply opacity-50 cursor-default;
}

.sheet-foot {
  @apply flex-none flex items-center h-[34px] px-3.5 border-t border-border text-[11.5px] text-text-tertiary;
}

.foot-error {
  @apply text-red;
}

.foot-warn {
  @apply text-orange;
}

/* ---- confirm ---- */
.confirm-overlay {
  @apply absolute inset-0 flex items-center justify-center;
  background: rgba(0, 0, 0, 0.25);
}

.confirm {
  @apply w-90 max-w-[90%] rounded-xl p-4.5;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.confirm-title {
  @apply text-[15px] font-semibold mb-2;
}

.confirm-body {
  @apply text-[13px] text-text-secondary mb-4;
}

.confirm-actions {
  @apply flex justify-end gap-2;
}

/* ---- import .env ---- */
.file-input {
  @apply hidden;
}

.import {
  @apply flex flex-col w-[620px] max-w-[90vw] max-h-[70vh] rounded-xl overflow-hidden;
  background: var(--bg-panel);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.18), 0 0 0 1px var(--border);
}

.import-title {
  @apply flex-none px-3.5 py-3 text-[13px] font-semibold border-b border-border;
}

.import-body {
  @apply flex-1 min-h-0 overflow-auto py-1;
}

.import-row {
  @apply grid items-center gap-2.5 px-3.5 py-1.5;
  grid-template-columns: 20px 180px minmax(0, 1fr) auto;
}

.import-secret {
  @apply w-[13px] h-[13px] rounded-[3px] border cursor-pointer inline-flex items-center justify-center text-white;
  border-color: var(--border-strong);
  background: var(--bg-panel);
}

.import-secret.on {
  @apply bg-orange border-orange;
}

.import-name {
  @apply text-[12.5px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.import-value {
  @apply text-[12.5px] text-text-secondary overflow-hidden text-ellipsis whitespace-nowrap;
}

.import-value.masked {
  @apply text-text-tertiary;
}

.import-new {
  @apply text-[11px] text-text-tertiary px-2;
}

.import-conflict {
  @apply inline-flex gap-0.5 p-0.5 rounded-md;
  background: var(--bg-inset);
}

.conflict-btn {
  @apply text-[11px] py-0.5 px-2 border-none rounded-md bg-transparent text-text-secondary cursor-pointer;
  font: inherit;
}

.conflict-btn.on {
  @apply bg-bg-panel text-text font-medium;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.import-empty {
  @apply px-3.5 py-3 text-xs text-text-tertiary;
}

.import-actions {
  @apply flex-none flex items-center gap-2 px-3.5 py-2.5 border-t border-border;
}

.import-summary {
  @apply flex-1 text-[11.5px] text-text-tertiary;
}
</style>
