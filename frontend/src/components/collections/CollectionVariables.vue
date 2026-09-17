<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Translation as I18nT } from 'vue-i18n'
import Icon from '../ui/Icon.vue'
import { useCollectionsStore } from '../../stores/collections'
import { useEnvironmentsStore } from '../../stores/environments'
import { useMessages } from '../../i18n'
import { VariableKind, type Variable } from '../../../bindings/json-inspector/internal/domain'

// The `{{tokens}}` a collection answers for the requests inside it. They are the environment's own
// kind of variable and stand over it: a level that names `baseUrl` means that value for everything
// below, which is what lets a collection be carried from one environment to another.
//
// A secret is not offered here, and that is the design rather than an omission: a collection is what
// gets exported and handed on, and its variables travel with it. The environment's own rows are shown
// under these ones, read-only, so that what the level resolves to is visible where it is decided.
const store = useCollectionsStore()
const environments = useEnvironmentsStore()
const { t } = useMessages()

// The sheet holds the set while it is open and writes it on Save: the footer's two buttons are a
// choice, and a set written on every keystroke would leave Cancel with nothing to cancel.
const rows = ref<Variable[]>([])
const dirty = ref(false)

function fromStore(): Variable[] {
  return (store.trail?.collection?.variables ?? []).map((variable) => ({ ...variable }))
}

watch(
  () => store.trail?.collection?.variables,
  () => {
    if (dirty.value) return
    rows.value = fromStore()
  },
  { immediate: true, deep: true }
)

// What this level calls itself in the scope column: a folder is a collection with a parent, and the
// two are worth telling apart in a row that says where the value lives.
const levelTag = computed(() =>
  (store.trail?.ancestors.length ?? 0) > 0 ? t('collections.folderTag') : t('collections.aCollection')
)

// The environment's rows, as the drawing shows them: the same shape with an id of their own, because
// they are looked at here and edited in the sheet that owns them.
const inherited = computed(() =>
  (environments.activeEnvironment?.vars ?? []).map((variable) => ({
    id: `env-${variable.id}`,
    variable,
  }))
)

function display(variable: Variable): string {
  if (variable.value) return variable.value
  return variable.hasValue ? '••••' : ''
}

function add() {
  dirty.value = true
  rows.value = [
    ...rows.value,
    {
      id: '',
      name: '',
      value: '',
      kind: VariableKind.VariableText,
      enabled: true,
      position: rows.value.length + 1,
      hasValue: false,
    },
  ]
}

function patch(row: Variable, change: Partial<Variable>) {
  dirty.value = true
  rows.value = rows.value.map((it) => (it.id === row.id ? { ...it, ...change } : it))
}

function remove(row: Variable) {
  dirty.value = true
  rows.value = rows.value.filter((it) => it.id !== row.id)
}

async function commit() {
  await store.saveVariables(store.selectedId ?? '', rows.value)
  dirty.value = false
}

function discard() {
  dirty.value = false
  rows.value = fromStore()
}

function openEnvironment() {
  environments.openSheet({ envId: environments.activeId ?? null, varName: '' })
}

defineExpose({ commit, discard })
</script>

<template>
  <div class="variables">
    <div class="table">
      <div class="head">
        <span>{{ t('collections.varName') }}</span>
        <span>{{ t('collections.varValue') }}</span>
        <span class="head-scope">{{ t('collections.columnScope') }}</span>
        <span></span>
      </div>

      <div v-for="row in rows" :key="row.id" class="row">
        <input
          :value="row.name"
          class="cell mono"
          :placeholder="t('collections.varName')"
          spellcheck="false"
          @input="patch(row, { name: ($event.target as HTMLInputElement).value })"
        />
        <input
          :value="row.value"
          class="cell mono own-value"
          :placeholder="t('collections.varValue')"
          spellcheck="false"
          @input="patch(row, { value: ($event.target as HTMLInputElement).value })"
        />
        <span class="tag own">{{ levelTag }}</span>
        <button class="drop" :title="t('common.delete')" @click="remove(row)">
          <Icon name="xmark" :size="13" :stroke-width="2.2" />
        </button>
      </div>

      <!-- What the environment answers for, in the same table and not editable here: a row that could
           be typed into would be a second place the same value is kept. It is drawn in the drawing's
           own dimmed tones — another surface, tertiary ink, a faded cross — because a reader has to
           see at a glance which rows this level owns. -->
      <div v-for="entry in inherited" :key="entry.id" class="row read-only">
        <span class="cell-static mono">{{ entry.variable.name }}</span>
        <span class="cell-static mono">{{ display(entry.variable) }}</span>
        <span class="tag env">{{ t('collections.scopeEnv') }}</span>
        <button class="drop" :title="t('collections.scopeEnvHint')" disabled>
          <Icon name="xmark" :size="13" :stroke-width="2.2" />
        </button>
      </div>
    </div>

    <button class="add" @click="add">
      <Icon name="plus" :size="14" :stroke-width="2.2" />
      {{ t('collections.addVariable') }}
    </button>

    <div class="hint">
      <span class="hint-text">
        <i18n-t keypath="collections.scopeHint" tag="span">
          <template #tag><span class="hint-tag">{{ t('collections.scopeEnv') }}</span></template>
        </i18n-t>
      </span>
      <button class="hint-action" @click="openEnvironment">{{ t('collections.openEnvironment') }}</button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.variables {
  @apply flex flex-col gap-3;
}

/* The table is its own block: the head stands 6px over the first row, and the rows 6px from each
   other — the sheet's 12px gap is between the table, the link and the hint. */
.table {
  @apply flex flex-col gap-1.5;
}

/* Key, value, where it comes from, and the row's own trailing button. The value takes what is left,
   because a URL is the longest thing in a table of variables. */
.head,
.row {
  @apply grid items-center gap-2;
  grid-template-columns: 130px minmax(0, 1fr) 76px 28px;
}

.head {
  @apply text-[11px] font-semibold uppercase tracking-[0.06em] text-text-tertiary;
}

.row {
  line-height: normal;
}

.cell {
  @apply min-w-0 w-full h-8 bg-bg-inset border border-border-strong rounded-[7px] px-[9px]
         text-[12.5px] text-text outline-none;
  font-family: var(--mono);
}

/* A value this level answers for is a literal, and the drawing inks it as one. */
.own-value {
  color: var(--tok-str);
}

.cell:focus {
  border-color: var(--accent);
}

/* A row of the environment: the same cell on a veiled surface and in tertiary ink, so what the level
   itself says reads first and what it inherits reads as context. */
.cell-static {
  @apply min-w-0 h-8 flex items-center overflow-hidden text-ellipsis whitespace-nowrap
         rounded-[7px] px-[9px] text-[12.5px] border border-border-strong bg-bg-hover
         text-text-tertiary;
}

.tag {
  @apply text-[11.5px] font-semibold text-center py-1 rounded-md;
}

.tag.own {
  @apply text-accent bg-accent-soft;
}

.tag.env {
  @apply text-text-tertiary bg-bg-hover;
}

.drop {
  @apply flex items-center justify-center w-7 h-7 rounded-md border-0 bg-transparent
         text-text-tertiary cursor-pointer;
}

.drop:hover:not(:disabled) {
  @apply bg-red-soft;
  color: var(--red-text);
}

.drop:disabled {
  @apply opacity-35 cursor-default;
}

/* «Add» is the drawing's link rather than a button: the sheet already has one filled button, and it
   is the footer's. */
.add {
  @apply flex items-center gap-[7px] self-start h-[30px] px-2 rounded-[7px] border-0
         bg-transparent text-[13px] font-medium text-accent cursor-pointer;
  font-family: inherit;
}

.add:hover {
  @apply bg-accent-soft;
}

.hint {
  @apply flex items-center gap-2.5 py-[11px] px-3 rounded-[9px] bg-bg-inset;
}

.hint-text {
  @apply flex-1 min-w-0 text-[12.5px] text-text-secondary;
  line-height: 1.5;
}

.hint-tag {
  @apply font-semibold;
}

.hint-action {
  @apply flex-none h-[30px] px-[11px] rounded-[7px] border border-border-strong bg-bg-inset
         text-[12.5px] font-medium text-text cursor-pointer;
  font-family: inherit;
}

.hint-action:hover {
  @apply bg-bg-hover;
}
</style>
