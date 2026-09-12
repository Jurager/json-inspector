<script setup lang="ts">
import { computed, nextTick, ref, type ComponentPublicInstance } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import DeleteEnvDialog from './DeleteEnvDialog.vue'
import ImportDialog from './ImportDialog.vue'
import {
  parseDotenv,
  useEnvironmentsStore,
  type ImportChoice,
  type Variable,
} from '../../stores/environments'
import { useRequestsStore } from '../../stores/requests'
import { parseTokens } from '../../lib/vars'
import { useSheetNotice } from '../../composables/useSheetNotice'

const envStore = useEnvironmentsStore()
const reqStore = useRequestsStore()
const { setNotice, clearNotice } = useSheetNotice()

const envId = computed(() => envStore.sheetEnvId)
const env = computed(() => envStore.environments.find((e) => e.id === envId.value) ?? null)
const isGlobals = computed(() => envId.value === null)

const renamingId = ref<string | null>(null)
const envDraft = ref('')
const renameInput = ref<HTMLInputElement | null>(null)
const renameInvalid = ref(false)

function setRenameInput(el: Element | ComponentPublicInstance | null) {
  renameInput.value = (el as HTMLInputElement | null) ?? null
}

function addEnv() {
  const id = envStore.addEnv()
  envStore.selectSheetEnv(id)
  startRename(id, true)
}

function startRename(id: string, selectAll = false) {
  const target = envStore.environments.find((e) => e.id === id)
  if (!target) return
  renamingId.value = id
  envDraft.value = target.name
  clearNotice()
  renameInvalid.value = false
  nextTick(() => {
    renameInput.value?.focus()
    if (selectAll) renameInput.value?.select()
  })
}

function commitRename(): boolean {
  const id = renamingId.value
  if (!id) return false
  const name = envDraft.value.trim()
  const others = envStore.environments.filter((e) => e.id !== id)
  if (!name || others.some((e) => e.name === name)) {
    renameInvalid.value = true
    setNotice(!name ? 'Имя окружения не может быть пустым' : 'Окружение с таким именем уже есть')
    renameInput.value?.focus()
    return false
  }
  envStore.renameEnv(id, name)
  renamingId.value = null
  renameInvalid.value = false
  clearNotice()
  return true
}

function commitRenameFrom(id: string) {
  if (renamingId.value !== id) return
  commitRename()
}

function cancelRename() {
  renamingId.value = null
  renameInvalid.value = false
  clearNotice()
}

function renameNext(dir: 1 | -1) {
  const list = envStore.environments
  const at = list.findIndex((e) => e.id === renamingId.value)
  const next = list[at + dir]
  if (!next) {
    cancelRename()
    return
  }
  if (commitRename()) startRename(next.id, true)
}

function onRenameKeydown(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    commitRename()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancelRename()
  } else if (e.key === 'Tab') {
    e.preventDefault()
    renameNext(e.shiftKey ? -1 : 1)
  }
}

function onEnvRowEnter(id: string) {
  if (envStore.sheetEnvId === id) startRename(id, true)
  else envStore.selectSheetEnv(id)
}

const confirming = ref<string | null>(null)

const confirmingName = computed(
  () => envStore.environments.find((e) => e.id === confirming.value)?.name ?? ''
)

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

// The file is read in the webview, not through a Go binding: parsing a text file needs no backend.
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
  // Conflicts default to "skip": an import must never overwrite a hand-set value without saying so
  // on the row.
  importEntries.value = parseDotenv(text).map((entry) => ({
    ...entry,
    mode: existingNames.value.has(entry.name) ? ('skip' as const) : ('replace' as const),
  }))
}

const existingNames = computed(
  () => new Set(envStore.rowsFor(envId.value).own.map((v: Variable) => v.name))
)

const importCount = computed(
  () => importEntries.value?.filter((e) => e.mode === 'replace' || !existingNames.value.has(e.name)).length ?? 0
)

function applyImport() {
  if (importEntries.value) envStore.importDotenv(envId.value, importEntries.value)
  importEntries.value = null
}

function cancelTop(): boolean {
  if (renamingId.value) {
    cancelRename()
    return true
  }
  if (importEntries.value) {
    importEntries.value = null
    return true
  }
  if (confirming.value) {
    confirming.value = null
    return true
  }
  return false
}

defineExpose({ cancelTop })
</script>

<template>
  <div class="sheet-side">
    <div class="side-label">Окружения</div>
    <div
      v-for="e in envStore.environments"
      :key="e.id"
      class="side-row"
      :class="{ active: e.id === envStore.sheetEnvId }"
      role="button"
      tabindex="0"
      title="Двойной клик или Enter — переименовать"
      @click="envStore.selectSheetEnv(e.id)"
      @keydown.enter="onEnvRowEnter(e.id)"
      @dblclick="startRename(e.id, true)"
    >
      <span class="side-dot" :class="{ on: e.id === envStore.activeId }"></span>
      <input
        v-if="renamingId === e.id"
        :ref="setRenameInput"
        v-model="envDraft"
        class="side-rename"
        :class="{ invalid: renameInvalid }"
        maxlength="40"
        spellcheck="false"
        @click.stop
        @keydown="onRenameKeydown"
        @blur="commitRenameFrom(e.id)"
      />
      <span v-else class="side-name">{{ e.name }}</span>
      <Icon v-if="e.readonly" name="lock" :size="11" class="side-lock" />
      <span v-else class="side-count mono">{{ e.vars.length }}</span>
    </div>

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
      <IconButton variant="bare" hint="Новое окружение" @click="addEnv">
        <Icon name="plus" :size="14" />
      </IconButton>
      <IconButton
        variant="bare"
        hint="Удалить окружение"
        :disabled="isGlobals"
        @click="isGlobals || askRemove(envStore.sheetEnvId as string)"
      >
        <Icon name="minus" :size="14" />
      </IconButton>
      <span class="side-foot-spacer"></span>
      <input
        ref="fileInput"
        type="file"
        accept=".env,text/plain"
        class="file-input"
        @change="onFileChosen"
      />
      <Button
        variant="quiet"
        :disabled="isGlobals"
        :title="isGlobals ? 'Импорт идёт в выбранное окружение, не в глобальные' : undefined"
        @click="pickFile"
      >
        Импорт .env
      </Button>
    </div>

    <ImportDialog
      v-if="importEntries"
      :entries="importEntries"
      :existing-names="existingNames"
      :target-name="env?.name ?? 'Глобальные'"
      :count="importCount"
      @cancel="importEntries = null"
      @apply="applyImport"
    />
    <DeleteEnvDialog
      v-if="confirming"
      :name="confirmingName"
      @cancel="confirming = null"
      @confirm="doRemove(confirming)"
    />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-side {
  @apply flex-none w-[232px] flex flex-col p-2 pb-0 border-r border-border;
  background: var(--bg-inset);
}

.side-label {
  @apply pt-1.5 px-2 pb-1.5 text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
  font-family: var(--mono);
}

.side-row {
  @apply flex items-center gap-2 w-full h-[30px] px-2 border-none rounded-md bg-transparent text-text text-[13px] text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.side-row:hover {
  @apply bg-bg-hover;
}

.side-row.active {
  @apply bg-accent-soft font-semibold;
}

.side-row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
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

.side-rename {
  @apply flex-1 min-w-0 h-[26px] box-border text-[13px] outline-none;
  margin-left: -8px;
  padding: 0 7px;
  border-radius: 6px;
  border: 1px solid var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
  background: var(--bg-panel);
  color: var(--text);
}

.side-rename.invalid {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
  box-shadow: 0 0 0 3px var(--red-soft);
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

.file-input {
  @apply hidden;
}
</style>
