<script setup lang="ts">
import { computed, nextTick, ref, type ComponentPublicInstance } from 'vue'
import Icon from '../ui/Icon.vue'
import { useEnvironmentsStore, type Variable } from '../../stores/environments'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'

const envStore = useEnvironmentsStore()
const { notice, setNotice, clearNotice } = useSheetNotice()

const envId = computed(() => envStore.editedEnvId)
const env = computed(() => envStore.environments.find((e) => e.id === envId.value) ?? null)
const isGlobals = computed(() => envId.value === null)

const isEnvLocked = computed(() => Boolean(env.value?.readonly) && !envStore.isUnlocked(envId.value))

const rows = computed(() => envStore.rowsFor(envId.value))
const vars = computed(() => rows.value.own)

const filter = ref('')
const matches = (v: Variable) => v.name.toLowerCase().includes(filter.value.trim().toLowerCase())

const filteredOwn = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? vars.value.filter(matches) : vars.value
})

const filteredInherited = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? rows.value.inherited.filter(matches) : rows.value.inherited
})

const revealed = ref<Set<string>>(new Set())

function isRevealed(v: Variable): boolean {
  return revealed.value.has(v.id)
}

function toggleReveal(v: Variable) {
  const next = new Set(revealed.value)
  if (next.has(v.id)) next.delete(v.id)
  else next.add(v.id)
  revealed.value = next
}

function isSecretMasked(v: Variable): boolean {
  return v.kind === 'secret' && !isRevealed(v)
}

function displayValue(v: Variable, scope: string | null): string {
  if (v.kind !== 'secret') return v.value
  return isRevealed(v) ? envStore.effectiveValue(scope, v) : '••••'
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
  if (isEnvLocked.value) return
  editing.value = { scope, varId: v.id, field }
  clearNotice()
  draft.value = field === 'name' ? v.name : envStore.effectiveValue(scope, v)
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
      setNotice('Имя не может быть пустым')
      return false
    }
    if (scopeVars.some((x) => x.id !== v.id && x.name === name)) {
      setNotice(ed.scope === null ? 'Такое имя уже есть в глобальных' : 'Такое имя уже есть в этом окружении')
      return false
    }
    envStore.updateVar(ed.scope, v.id, { name })
  } else {
    envStore.updateVar(ed.scope, v.id, { value: draft.value })
  }
  editing.value = null
  clearNotice()
  return true
}

function cancel() {
  editing.value = null
  clearNotice()
}

function moveTo(v: Variable, field: 'name' | 'value') {
  const at = filteredOwn.value.indexOf(v)
  const next = filteredOwn.value[at + 1]
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

function openGlobal(g: Variable, edit = false) {
  envStore.editEnv(null)
  nextTick(() => {
    document.getElementById('var-' + g.id)?.scrollIntoView({ block: 'center' })
    if (edit) {
      startEdit(g, 'name', null)
      return
    }
    flashId.value = g.id
    setTimeout(() => {
      if (flashId.value === g.id) flashId.value = null
    }, 1200)
  })
}

function addVar() {
  if (isEnvLocked.value) return
  const id = envStore.addVar(envId.value, { name: '', value: '' })
  const created = vars.value.find((v) => v.id === id)
  if (created) startEdit(created, 'name')
}

function removeVar(v: Variable) {
  if (isEnvLocked.value) return
  envStore.removeVar(envId.value, v.id)
}

function toggleKind(v: Variable) {
  if (isEnvLocked.value) return
  envStore.updateVar(envId.value, v.id, { kind: v.kind === 'secret' ? 'text' : 'secret' })
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
  <div class="sheet-main">
    <div class="sheet-toolbar">
      <span class="toolbar-spacer"></span>
      <Button
        v-if="isEnvLocked"
        title="Окружение только для чтения"
        @click="envStore.unlock(envStore.editedEnvId as string)"
      >
        Разблокировать
      </Button>
      <div class="filter">
        <Icon name="search" :size="12" />
        <Input v-model="filter" variant="bare" size="sm" class="flex-1 min-w-0" placeholder="Фильтр по имени" spellcheck="false" />
      </div>
    </div>

    <div class="table-head">
      <div>Переменная</div>
      <div>Значение</div>
      <div>Тип</div>
      <div></div>
    </div>

    <div class="table-body">
      <div
        v-for="v in filteredOwn"
        :key="v.id"
        :id="'var-' + v.id"
        class="row"
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
              displayValue(v, envId)
            }}</span>
            <IconButton
              v-if="v.kind === 'secret'"
              size="sm"
              :hint="isRevealed(v) ? 'Скрыть значение' : 'Показать значение'"
              @click.stop="toggleReveal(v)"
            >
              <Icon :name="isRevealed(v) ? 'eye-off' : 'eye'" :size="13" />
            </IconButton>
          </template>
        </div>

        <div class="cell">
          <span v-if="isGlobals" class="tag tag-global">глобальная</span>
          <button
            v-else
            class="tag"
            :class="v.kind === 'secret' ? 'tag-secret' : 'tag-text'"
            :disabled="isEnvLocked"
            :title="isEnvLocked ? 'Окружение только для чтения' : 'Переключить тип'"
            @click="toggleKind(v)"
          >
            {{ v.kind === 'secret' ? 'секрет' : 'текст' }}
          </button>
        </div>

        <div class="cell cell-action">
          <IconButton v-if="!isEnvLocked" variant="danger" size="sm" hint="Удалить" @click="removeVar(v)">
            <Icon name="trash" :size="13" />
          </IconButton>
        </div>
      </div>

      <div v-if="filteredOwn.length === 0 && !isGlobals" class="table-empty">
        {{ vars.length === 0 ? 'Своих переменных пока нет' : 'Ничего не найдено' }}
      </div>
      <div v-else-if="filteredOwn.length === 0" class="table-empty">
        {{ vars.length === 0 ? 'Переменных пока нет' : 'Ничего не найдено' }}
      </div>

      <button class="new-row" :disabled="isEnvLocked" @click="addVar">
        <Icon name="plus" :size="13" />
        <span>Новая переменная</span>
      </button>

      <!-- Inherited globals shown inline, so nobody has to guess where a variable came from. -->
      <template v-if="filteredInherited.length">
        <div class="group-head">
          <span class="group-title">Наследуется из глобальных</span>
          <span class="group-count mono">{{ filteredInherited.length }}</span>
          <span class="group-hint">действуют во всех окружениях</span>
        </div>
        <div
          v-for="g in filteredInherited"
          :key="g.id"
          class="row inherited"
          :class="{ overridden: g.overridden, flash: flashId === g.id }"
          role="button"
          tabindex="0"
          title="Перейти к переменной в «Глобальных»"
          @click="openGlobal(g)"
          @keydown.enter="openGlobal(g)"
        >
          <div class="cell cell-name">
            <Icon name="inherit" :size="11" class="inherit-icon" />
            <span class="cell-text mono" :title="g.overridden ? 'Перекрыта переменной окружения' : g.name">{{
              g.name
            }}</span>
          </div>
          <div class="cell cell-value">
            <span class="cell-text mono">{{ displayValue(g, null) }}</span>
          </div>
          <div class="cell">
            <span v-if="g.overridden" class="tag tag-overridden">перекрыта</span>
            <span v-else class="tag tag-global">глобальная</span>
          </div>
          <div class="cell cell-action">
            <IconButton
              v-if="!isEnvLocked"
              size="sm"
              hint="Править глобальную переменную"
              @click.stop="openGlobal(g, true)"
            >
              <Icon name="pencil" :size="13" />
            </IconButton>
          </div>
        </div>
      </template>
    </div>

    <div class="sheet-foot">
      <span v-if="notice" class="foot-error">{{ notice }}</span>
      <span v-else-if="!envStore.isKeychainAvailable" class="foot-warn">
        Связка ключей недоступна — секреты не сохранятся после выхода
      </span>
      <span v-else>Секреты хранятся в связке ключей macOS и не попадают в экспорт коллекции</span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

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

.toolbar-spacer {
  @apply flex-1;
}

.table-head,
.row {
  @apply grid items-stretch gap-0 px-3.5;
  grid-template-columns: 200px minmax(0, 1fr) 96px 34px;
}

.table-head {
  @apply flex-none h-[30px] mt-2 border-b border-border text-[10px] uppercase tracking-[0.08em] text-text-tertiary;
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
  @apply min-w-0 flex items-center cursor-text;
}

.cell-name {
  @apply pr-3;
}

.cell-value {
  @apply pr-3;
}

.cell-text {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px];
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
  @apply text-xs font-semibold py-px px-[5px] rounded-sm border-none;
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

.tag-overridden {
  @apply text-text-tertiary;
  background: var(--bg-hover);
}

.group-head {
  @apply flex items-center gap-2 h-[30px] px-3.5;
  background: var(--bg-inset);
}

.group-title {
  @apply text-[10px] uppercase tracking-[0.08em] text-text-secondary;
}

.group-count {
  @apply text-xs text-text-tertiary;
}

.group-hint {
  @apply flex-1 text-right text-xs text-text-tertiary;
}

.row.inherited {
  background: color-mix(in srgb, var(--purple) 3%, transparent);
}

.row.inherited:hover {
  background: color-mix(in srgb, var(--purple) 7%, transparent);
}

.inherit-icon {
  @apply flex-none mr-1.5 text-purple;
}

.row.inherited.overridden .cell-text {
  @apply text-text-tertiary line-through;
}

.row.inherited.overridden .inherit-icon {
  @apply opacity-40;
}

.row.inherited .cell-text {
  @apply text-text-secondary;
}

.row.flash {
  box-shadow: inset 0 0 0 2px var(--accent);
}

.cell-action {
  @apply justify-end;
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
  @apply flex-none flex items-center py-3 px-4 border-t border-border text-xs text-text-tertiary;
}

.foot-error {
  @apply text-red;
}

.foot-warn {
  @apply text-orange;
}
</style>
