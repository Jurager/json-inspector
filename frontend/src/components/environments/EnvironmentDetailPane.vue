<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import ColorSwatches from '../ui/ColorSwatches.vue'
import Segment from '../ui/Segment.vue'
import VariablesTable from './VariablesTable.vue'
import { ENVIRONMENT_COLORS, ENVIRONMENT_TINTS } from './palette'
import { useEnvironmentsStore } from '../../stores/environments'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'

// One environment: what it is called, what colour it wears, whether it is open to editing, the
// variables in it and the globals they inherit — and, at the foot, the one thing that takes it all
// away again.
//
// The globals are the same pane with less of it: they are not an environment, hold no colour of
// their own and cannot be closed to editing, so those two controls are not drawn rather than drawn
// disabled.
const emit = defineEmits<{ (e: 'import'): void; (e: 'danger'): void }>()

const envStore = useEnvironmentsStore()
const { setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

// One name per swatch, so a colour is readable without a legend. A computed and not a plain object: a
// map made once would keep the words of the language it was made in.
const colorLabels = computed<Record<string, string>>(() => ({
  green: t('environments.colorGreen'),
  orange: t('environments.colorOrange'),
  red: t('environments.colorRed'),
  purple: t('environments.colorPurple'),
}))

const env = computed(() => envStore.environments.find((e) => e.id === envStore.editedEnvId) ?? null)
const isGlobals = computed(() => envStore.editedEnvId === null)

const name = ref('')
const invalid = ref(false)
const nameInput = ref<HTMLInputElement | null>(null)

// The field follows what the pane is showing: moving between rows is not an edit, and a name that
// was refused leaves nothing behind once another environment is on screen.
watch([() => env.value?.id, () => env.value?.name, isGlobals], syncName, { immediate: true })

function syncName() {
  name.value = isGlobals.value ? t('environments.globals') : (env.value?.name ?? '')
  invalid.value = false
}

function refuse(message: string) {
  invalid.value = true
  setNotice(message)
  nextTick(() => nameInput.value?.select())
}

// Renaming happens here now: Enter and leaving the field both commit, Escape puts the stored name
// back, and a refused one keeps the field — with what was typed still in it — so it can be fixed.
async function commit() {
  const target = env.value
  if (!target) return
  const wanted = name.value.trim()

  if (wanted === target.name) {
    invalid.value = false
    clearNotice()
    return
  }
  if (!wanted) {
    refuse(t('environments.nameEmpty'))
    return
  }
  if (envStore.environments.some((e) => e.id !== target.id && e.name === wanted)) {
    refuse(t('environments.nameTaken'))
    return
  }

  invalid.value = false
  clearNotice()
  try {
    await envStore.renameEnv(target.id, wanted)
  } catch (error) {
    refuse(describeFailure(error))
  }
}

function revert() {
  name.value = env.value?.name ?? ''
  invalid.value = false
  clearNotice()
}

const accessOptions = computed(() => [
  { value: 'editable', label: t('environments.accessEditable') },
  { value: 'readonly', label: t('environments.accessReadonly') },
])

const access = computed(() => (env.value?.readonly ? 'readonly' : 'editable'))

// Making an environment read-only is this write and nothing else: the window reads the flag back and
// stops drawing the cells as editable. Nothing on the Go side refuses a write to a read-only
// environment, so the lock is the window's promise — which is why it is one switch in one place.
function pickAccess(value: string) {
  const target = env.value
  if (!target) return
  void envStore.setEnvReadonly(target.id, value === 'readonly')
}

function pickColor(color: string) {
  const target = env.value
  if (!target) return
  void envStore.setEnvColor(target.id, color)
}

const dangerNote = computed(() =>
  isGlobals.value ? t('environments.dangerGlobalsNote') : t('environments.dangerDeleteNote')
)

const dangerLabel = computed(() =>
  isGlobals.value ? t('environments.dangerClear') : t('environments.dangerDelete')
)

// Escape belongs to the innermost thing on screen: the cell being edited first, and the window after
// it. The table holds the cell; the sheet asks the pane, and the pane asks the table.
const table = ref<InstanceType<typeof VariablesTable> | null>(null)

function cancelTop(): boolean {
  return table.value?.cancelTop() ?? false
}

defineExpose({ cancelTop })
</script>

<template>
  <div class="pane">
    <div class="pane-body">
      <div class="pair">
        <label class="field">
          <span class="field-label">{{ t('environments.nameLabel') }}</span>
          <input
            ref="nameInput"
            v-model="name"
            class="input"
            :class="{ invalid }"
            :readonly="isGlobals"
            maxlength="40"
            spellcheck="false"
            @keydown.enter="commit"
            @keydown.esc="revert"
            @blur="commit"
          />
        </label>

        <div v-if="env" class="field tight">
          <span class="field-label">{{ t('environments.colorLabel') }}</span>
          <ColorSwatches
            :value="env.color ?? ''"
            :colors="ENVIRONMENT_COLORS"
            :tints="ENVIRONMENT_TINTS"
            :labels="colorLabels"
            @pick="pickColor"
          />
        </div>
      </div>

      <div v-if="env" class="field">
        <span class="field-label">{{ t('environments.accessLabel') }}</span>
        <Segment class="access" :options="accessOptions" :value="access" @pick="pickAccess" />
      </div>

      <VariablesTable ref="table" :locked="Boolean(env?.readonly)" @import="emit('import')" />
    </div>

    <div class="foot">
      <span class="danger-note">{{ dangerNote }}</span>
      <button
        type="button"
        class="danger"
        :disabled="isGlobals && envStore.globals.length === 0"
        @click="emit('danger')"
      >
        {{ dangerLabel }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The pane is a column with one scrolling part and one pinned part: the content moves, the buttons
   stay where the eye last saw them. */
.pane {
  @apply flex-1 min-w-0 flex flex-col min-h-0;
}

.pane-body {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-[18px];
  padding: 18px 20px;
}

.pair {
  @apply flex gap-3.5 items-end;
}

.field {
  @apply flex flex-col gap-1.5 min-w-0;
}

/* Only the field that stands beside another one takes the room: the colour group is as wide as its
   swatches, and a field that is alone in the column is as tall as what is in it. */
.pair > .field {
  @apply flex-1;
}

.pair > .field.tight {
  @apply flex-none;
}

.field-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.input {
  @apply w-full h-[34px] px-[11px] rounded-lg text-[13.5px] text-text outline-none;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.input:focus,
.input.invalid {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

/* A name that was refused keeps the field lit, in the colour of the refusal. */
.input.invalid {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
  box-shadow: 0 0 0 3px var(--red-soft);
}

.input[readonly] {
  @apply text-text-secondary cursor-default;
}

.access {
  max-width: 260px;
}

/* The foot is pinned to the leaf and no longer scrolls away with the form. It is tall enough that
   the button in it has air above and below rather than sitting on the border, and its height is what
   keeps it level with the rail's own: that foot is 10px of padding around a 28px button, which puts
   the button's centre 24px above the bottom, and this one centres whatever stands in it. */
.foot {
  gap: 12px;
  @apply flex-none flex items-center h-12 px-5;
  border-top: 1px solid var(--border);
}


.danger-note {
  @apply flex-1 min-w-0 text-[12.5px] text-text-tertiary;
}

.danger {
  @apply flex-none h-[30px] px-3 rounded-[7px] cursor-pointer bg-transparent;
  font-family: inherit;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--red-text);
  border: 1px solid var(--red-soft);
}

.danger:hover:not(:disabled) {
  @apply bg-red-soft;
}

.danger:disabled {
  @apply opacity-50 cursor-default;
}
</style>
