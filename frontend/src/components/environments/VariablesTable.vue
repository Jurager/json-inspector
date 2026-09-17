<script setup lang="ts">
import { computed, nextTick, ref, watch, type ComponentPublicInstance } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import { useEnvironmentsStore } from '../../stores/environments'
import { VariableKind } from '../../../bindings/json-inspector/internal/domain'
import type { Variable } from '../../../bindings/json-inspector/internal/domain'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'
import { isVariableName } from '../../lib/vars'

// The variables of the environment on screen, and the globals it inherits underneath them. Both are
// cards: a header of column names, a line per variable, and a line to add one.
//
// Everything is edited in place — the name, the value and the kind are the row itself — so the
// table's keyboard is the editor's: Enter commits and moves on, Escape puts the old value back, Tab
// walks down the names. Reading it is meant to be possible without any of that, which is why a row
// shows what it holds before anybody clicks it.
const props = defineProps<{ locked: boolean }>()

const emit = defineEmits<{ (e: 'import'): void }>()

const envStore = useEnvironmentsStore()
const { notice, setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

const envId = computed(() => envStore.editedEnvId)
const isGlobals = computed(() => envId.value === null)

const rows = computed(() => envStore.rowsFor(envId.value))
const vars = computed(() => rows.value.own)

function isRevealed(v: Variable): boolean {
  return envStore.isRevealed(v.id)
}

function toggleReveal(v: Variable) {
  if (envStore.isRevealed(v.id)) {
    envStore.hide(v.id)
    return
  }
  void envStore.reveal(v.id)
}

function isSecretMasked(v: Variable): boolean {
  return v.kind === VariableKind.VariableSecret && !isRevealed(v)
}

function displayValue(v: Variable): string {
  if (v.kind !== VariableKind.VariableSecret) return v.value ?? ''
  return isRevealed(v) ? envStore.effectiveValue(v) : '••••'
}

interface Editing {
  scope: string | null
  varId: string
  field: 'name' | 'value'
}

const editing = ref<Editing | null>(null)
const draft = ref('')
const cellInput = ref<HTMLInputElement | null>(null)

function setCellInput(el: Element | ComponentPublicInstance | null) {
  cellInput.value = (el as HTMLInputElement | null) ?? null
}

function startEdit(v: Variable, field: 'name' | 'value', scope: string | null = envId.value) {
  if (props.locked) return
  editing.value = { scope, varId: v.id, field }
  clearNotice()
  draft.value = field === 'name' ? v.name : envStore.effectiveValue(v)
  nextTick(() => cellInput.value?.focus())
}

function commitFrom(v: Variable, field: 'name' | 'value') {
  const ed = editing.value
  if (!ed || ed.varId !== v.id || ed.field !== field) return
  commit()
}

function isEditingCell(v: Variable, field: 'name' | 'value', scope: string | null): boolean {
  const ed = editing.value
  return Boolean(ed && ed.varId === v.id && ed.field === field && ed.scope === scope)
}

function commit(): boolean {
  const ed = editing.value
  if (!ed) return false
  const scopeVars = envStore.varsOf(ed.scope)
  const v = scopeVars.find((x) => x.id === ed.varId)
  if (!v) {
    editing.value = null
    return false
  }
  if (ed.field === 'name') {
    const name = draft.value.trim()
    if (!name) {
      setNotice(t('environments.nameRequired'))
      return false
    }
    if (!isVariableName(name)) {
      setNotice(t('environments.nameInvalid'))
      return false
    }
    if (scopeVars.some((x) => x.id !== v.id && x.name === name)) {
      setNotice(ed.scope === null ? t('environments.nameTakenGlobals') : t('environments.nameTakenHere'))
      return false
    }
    // A refusal from Go is shown like one of ours: the write is where a name the database already
    // holds is caught, and a rejected call would otherwise leave the row looking at its old name
    // with nothing to say about why.
    void envStore.updateVar(ed.scope, v.id, { name }).catch(showRefusal)
  } else {
    void envStore.updateVar(ed.scope, v.id, { value: draft.value }).catch(showRefusal)
  }
  editing.value = null
  clearNotice()
  return true
}

function showRefusal(error: unknown) {
  setNotice(describeFailure(error))
}

function cancel() {
  editing.value = null
  clearNotice()
}

function moveTo(v: Variable, field: 'name' | 'value') {
  const at = vars.value.indexOf(v)
  const next = vars.value[at + 1]
  if (field === 'value' && next) startEdit(next, 'name')
  else if (field === 'value') cancel()
  else startEdit(v, 'value')
}

function onCellKeydown(e: KeyboardEvent, v: Variable, field: 'name' | 'value', scope: string | null) {
  if (e.key === 'Enter') {
    e.preventDefault()
    if (commit() && field === 'name') startEdit(v, 'value', scope)
  } else if (e.key === 'Escape') {
    e.preventDefault()
    cancel()
  } else if (e.key === 'Tab' && scope !== null) {
    e.preventDefault()
    if (commit()) moveTo(v, field)
  }
}

const flashId = ref<string | null>(null)

function flash(id: string) {
  flashId.value = id
  setTimeout(() => {
    if (flashId.value === id) flashId.value = null
  }, 1200)
}

function openGlobal(g: Variable, edit = false) {
  envStore.editEnv(null)
  nextTick(() => {
    document.getElementById('var-' + g.id)?.scrollIntoView({ block: 'center' })
    if (edit) {
      startEdit(g, 'name', null)
      return
    }
    flash(g.id)
  })
}

// A `{{token}}` the window was clicked on asks the sheet to open on the variable it names. The store
// carries that request and this is where it is answered: the scope switches if the name lives in
// another one, the row is brought into view and blinked. A name that is not there yet is remembered
// once — the request builder adds missing variables and asks for the sheet in the same breath.
const pendingName = ref('')

function revealName(name: string) {
  const found = vars.value.find((v) => v.name === name)
  if (!found) {
    pendingName.value = name
    return
  }
  pendingName.value = ''
  document.getElementById('var-' + found.id)?.scrollIntoView({ block: 'center' })
  flash(found.id)
}

watch(
  () => envStore.sheetFocus,
  (focus) => {
    if (!focus || !focus.varName) return
    const wanted = envStore.takeFocus()
    if (!wanted) return
    if (wanted.envId !== envId.value) envStore.editEnv(wanted.envId)
    nextTick(() => revealName(wanted.varName))
  },
  { immediate: true }
)

watch(vars, () => {
  if (pendingName.value) revealName(pendingName.value)
})

async function addVar() {
  if (props.locked) return
  const id = await envStore.addVar(envId.value, { name: '', value: '' })
  const created = vars.value.find((v) => v.id === id)
  if (created) startEdit(created, 'name')
}

function removeVar(v: Variable) {
  if (props.locked) return
  void envStore.removeVar(envId.value, v.id)
}

function toggleKind(v: Variable) {
  if (props.locked) return
  const next =
    v.kind === VariableKind.VariableSecret ? VariableKind.VariableText : VariableKind.VariableSecret
  void envStore.updateVar(envId.value, v.id, { kind: next })
}

function cancelTop(): boolean {
  if (editing.value) {
    cancel()
    return true
  }
  return false
}

defineExpose({ cancelTop })
</script>

<template>
  <div class="variables">
    <div class="section">
      <div class="section-head">
        <span class="section-label">{{ t('environments.variablesLabel') }}</span>
        <span class="section-count mono">{{ vars.length }}</span>
        <span class="head-spacer"></span>
        <!-- Importing is a write like any other, so a closed environment does not offer it: the
             switch that opens it is one row above, and a button that filled a read-only environment
             with a file would be the same broken promise as one that added a variable. -->
        <button
          type="button"
          class="section-action"
          :disabled="props.locked"
          :title="props.locked ? t('environments.locked') : undefined"
          @click="emit('import')"
        >
          {{ t('environments.importEnv') }}
        </button>
      </div>

      <div class="card">
        <div class="grid card-head">
          <div>{{ t('environments.variable') }}</div>
          <div>{{ t('environments.value') }}</div>
          <div>{{ t('environments.type') }}</div>
          <div></div>
        </div>

        <div
          v-for="v in vars"
          :key="v.id"
          :id="'var-' + v.id"
          class="grid row"
          :class="{ flash: flashId === v.id }"
        >
          <div class="cell cell-name" @click="startEdit(v, 'name')">
            <input
              v-if="isEditingCell(v, 'name', envId)"
              :ref="setCellInput"
              v-model="draft"
              class="cell-input mono"
              :class="{ invalid: notice }"
              spellcheck="false"
              @keydown="onCellKeydown($event, v, 'name', envId)"
              @blur="commitFrom(v, 'name')"
            />
            <span v-else class="cell-text mono" :title="v.name">{{ v.name }}</span>
          </div>

          <div class="cell cell-value" @click="startEdit(v, 'value')">
            <input
              v-if="isEditingCell(v, 'value', envId)"
              :ref="setCellInput"
              v-model="draft"
              class="cell-input mono"
              :type="isSecretMasked(v) ? 'password' : 'text'"
              spellcheck="false"
              @keydown="onCellKeydown($event, v, 'value', envId)"
              @blur="commitFrom(v, 'value')"
            />
            <template v-else>
              <span class="cell-text mono" :class="{ masked: isSecretMasked(v) }">{{
                displayValue(v)
              }}</span>
              <IconButton
                v-if="v.kind === VariableKind.VariableSecret"
                size="xl"
                :hint="isRevealed(v) ? t('environments.hideValue') : t('environments.showValue')"
                @click.stop="toggleReveal(v)"
              >
                <Icon :name="isRevealed(v) ? 'eye-off' : 'eye'" :size="14" />
              </IconButton>
            </template>
          </div>

          <!-- The kind is the same control in every scope: a global is a `{{token}}` that applies
               everywhere, and nothing about that keeps it from being a secret — the mask, the eye and
               the export rules all read the kind, not the scope. -->
          <div class="cell">
            <button
              class="tag"
              :class="v.kind === VariableKind.VariableSecret ? 'tag-secret' : 'tag-text'"
              :disabled="props.locked"
              :title="props.locked ? t('environments.locked') : t('environments.toggleKind')"
              @click="toggleKind(v)"
            >
              {{
                v.kind === VariableKind.VariableSecret
                  ? t('environments.secret')
                  : t('environments.text')
              }}
            </button>
          </div>

          <div class="cell cell-action">
            <IconButton
              v-if="!props.locked"
              variant="danger"
              size="xl"
              :hint="t('common.delete')"
              @click="removeVar(v)"
            >
              <Icon name="trash" :size="14" :stroke-width="1.8" />
            </IconButton>
          </div>
        </div>

        <div v-if="vars.length === 0" class="table-empty">
          {{ isGlobals ? t('environments.noVars') : t('environments.noOwnVars') }}
        </div>

        <button class="new-row" :disabled="props.locked" @click="addVar">
          <Icon name="plus" :size="14" />
          <span>{{ t('environments.newVariable') }}</span>
        </button>
      </div>

      <!-- One line for two things: what the sheet says about secrets by default, and what it says
           instead when an edit was refused. -->
      <span class="note" :class="{ bad: !!notice }">{{ notice || t('environments.secretsNote') }}</span>
    </div>

    <!-- Inherited globals shown beside the environment's own, so nobody has to guess where a name
         came from — and the pencil goes to the scope the name really lives in. -->
    <div v-if="rows.inherited.length" class="section">
      <div class="section-head">
        <span class="section-label">{{ t('environments.inheritedFromGlobals') }}</span>
        <span class="section-count mono">{{ rows.inherited.length }}</span>
        <span class="head-spacer"></span>
        <span class="section-hint">{{ t('environments.applyEverywhere') }}</span>
      </div>

      <div class="card">
        <button
          v-for="g in rows.inherited"
          :key="g.id"
          type="button"
          :id="'var-' + g.id"
          class="grid row inherited"
          :class="{ overridden: g.overridden, flash: flashId === g.id }"
          :title="t('environments.goToGlobal')"
          @click="openGlobal(g)"
        >
          <div class="cell cell-name">
            <Icon name="inherit" :size="12" class="inherit-icon" />
            <span
              class="cell-text mono"
              :title="g.overridden ? t('environments.overridden') : g.name"
              >{{ g.name }}</span
            >
          </div>
          <div class="cell cell-value">
            <span class="cell-text mono">{{ displayValue(g) }}</span>
          </div>
          <div class="cell">
            <span v-if="g.overridden" class="tag tag-overridden">{{
              t('environments.overriddenShort')
            }}</span>
            <span v-else class="tag tag-global">{{ t('environments.global') }}</span>
          </div>
          <div class="cell cell-action">
            <IconButton
              size="xl"
              :hint="t('environments.editGlobal')"
              @click.stop="openGlobal(g, true)"
            >
              <Icon name="pencil" :size="13" />
            </IconButton>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.variables {
  @apply flex flex-col gap-[18px];
}

.section {
  @apply flex flex-col gap-2;
}

.section-head {
  @apply flex items-center gap-2.5;
}

.section-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.section-count {
  @apply text-[11px] text-text-tertiary;
}

.section-hint {
  @apply text-[11.5px] text-text-tertiary;
}

.section-action {
  @apply h-[26px] px-[9px] border-0 rounded-md cursor-pointer bg-transparent
         text-text-secondary;
  font-family: inherit;
  font-size: 12.5px;
  font-weight: 500;
}

.section-action:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}

.section-action:disabled {
  @apply opacity-50 cursor-default;
}

/* The table is a card: the header is inside its frame and the rows end on its border. */
.card {
  @apply border border-border rounded-[10px] overflow-hidden;
}

.grid {
  @apply grid items-stretch;
  grid-template-columns: 190px minmax(0, 1fr) 100px 46px;
  padding: 0 12px;
}

.card-head {
  @apply h-8 items-center bg-bg-inset text-[11px] font-semibold uppercase tracking-[0.06em]
         text-text-tertiary;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.row {
  @apply h-10 border-b;
  border-color: color-mix(in srgb, var(--border) 60%, transparent);
}

.row:hover {
  @apply bg-bg-hover;
}

.cell {
  @apply flex items-center min-w-0;
}

.cell-name,
.cell-value {
  padding-right: 12px;
}

.cell-name {
  gap: 7px;
}

.cell-value {
  gap: 6px;
}

.cell-text {
  @apply min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px] text-text;
}

.cell-text.masked {
  @apply text-text-tertiary;
}

.cell-input {
  @apply w-full text-[12.5px] px-1 py-0.5 rounded-sm outline-none;
  background: var(--bg-inset);
  border: 1px solid var(--accent);
}

.cell-input.invalid {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
  box-shadow: 0 0 0 2px var(--red-soft);
}

.cell-action {
  @apply justify-end;
}

.tag {
  @apply flex-none text-[11px] font-semibold cursor-pointer;
  padding: 3px 7px;
  border-radius: 5px;
}

/* The kind of a read-only environment is a label and not a switch. */
.tag:disabled {
  @apply cursor-default;
}

/* The kind is a switch as well as a label: the chip is where a variable is made a secret, and where
   a secret is made ordinary again. */
.tag-text {
  @apply text-text-secondary bg-bg-hover;
}

.tag-secret {
  @apply bg-red-soft;
  color: var(--red-text);
}

.tag-global {
  @apply text-purple;
  background: color-mix(in srgb, var(--purple) 13%, transparent);
}

.tag-overridden {
  @apply text-text-tertiary bg-bg-hover;
}

.new-row {
  @apply flex items-center w-full h-10 gap-[7px] border-0 border-b bg-transparent text-left
         text-text-tertiary cursor-pointer;
  font-family: inherit;
  font-size: 12.5px;
  padding: 0 12px;
  border-color: color-mix(in srgb, var(--border) 60%, transparent);
}

.new-row:hover:not(:disabled) {
  @apply text-accent bg-bg-hover;
}

.new-row:disabled {
  @apply opacity-50 cursor-default;
}

.table-empty {
  @apply px-3 py-3 text-xs text-text-tertiary;
}

/* An inherited row is a link to the scope the name really lives in and not an editable one, so it is
   a button — and it wears the same grid and padding as the rows above it. It is tinted, and a name an
   environment also holds is struck through: the overriding is the fact worth seeing. */
.row.inherited {
  @apply w-full border-0 text-left cursor-pointer;
  font-family: inherit;
  background: color-mix(in srgb, var(--purple) 3%, transparent);
}

.row.inherited:hover {
  background: color-mix(in srgb, var(--purple) 7%, transparent);
}

.inherit-icon {
  @apply flex-none text-purple;
}

.row.overridden .cell-text {
  @apply text-text-tertiary line-through;
}

.row.overridden .inherit-icon {
  @apply opacity-40;
}

/* The row a `{{token}}` was clicked on, blinking once so the eye can find it. */
.row.flash {
  box-shadow: inset 0 0 0 2px var(--accent);
}

.note {
  @apply text-[11.5px] text-text-tertiary;
}

.note.bad {
  @apply text-red;
}
</style>
