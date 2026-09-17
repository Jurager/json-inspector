<script setup lang="ts">
import { computed, ref } from 'vue'
import ColorSwatches from '../ui/ColorSwatches.vue'
import Segmented from './Segmented.vue'
import { DEFAULT_ENVIRONMENT_COLOR, ENVIRONMENT_COLORS, ENVIRONMENT_TINTS } from './palette'
import { useEnvironmentsStore } from '../../stores/environments'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'

// The create form: a name, a colour, and what to fill the new environment from. The three are one
// action because they are one question — an environment usually starts as a copy of another one, and
// asking for the name first would mean asking for the rest under a heading that already exists.
const props = defineProps<{ pendingCount: number }>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'created'): void
  (e: 'import'): void
}>()

const envStore = useEnvironmentsStore()
const { notice, setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

// One name per swatch, so a colour is readable without a legend. A computed and not a plain object: a
// map made once would keep the words of the language it was made in.
const colorLabels = computed<Record<string, string>>(() => ({
  green: t('environments.colorGreen'),
  orange: t('environments.colorOrange'),
  red: t('environments.colorRed'),
  purple: t('environments.colorPurple'),
}))

const name = ref('')
const color = ref(DEFAULT_ENVIRONMENT_COLOR)
// Empty is the blank environment; anything else is an id to copy from.
const base = ref('')

const bases = computed(() => [
  { value: '', label: t('environments.baseEmpty') },
  ...envStore.environments.map((e) => ({ value: e.id, label: e.name })),
])

const baseEnv = computed(() => envStore.environments.find((e) => e.id === base.value) ?? null)

// What the Variables card says before there is anything to show. It is the one place the form can
// answer "what will I get", and the answer differs by which base was chosen.
const note = computed(() => {
  const parts: string[] = []
  const chosen = baseEnv.value

  if (chosen) {
    parts.push(t('environments.createFromBase', { n: chosen.vars.length, name: chosen.name }))
    if (chosen.vars.some((v) => v.kind === 'secret')) parts.push(t('environments.createSecretsNote'))
  } else {
    parts.push(t('environments.createEmptyNote'))
  }

  // A .env file chosen before the environment exists has nowhere to go yet: it is written in as soon
  // as the form makes one, and the form says so rather than looking like it forgot.
  if (props.pendingCount > 0) parts.push(t('environments.importPending', { n: props.pendingCount }))

  return parts.join(' ')
})

async function create() {
  const wanted = name.value.trim()
  if (!wanted) {
    setNotice(t('environments.nameEmpty'))
    return
  }
  if (envStore.environments.some((e) => e.name === wanted)) {
    setNotice(t('environments.nameTaken'))
    return
  }

  try {
    await envStore.createEnv({ name: wanted, color: color.value, startFrom: base.value })
  } catch (error) {
    setNotice(describeFailure(error))
    return
  }
  clearNotice()
  emit('created')
}

function cancel() {
  clearNotice()
  emit('cancel')
}
</script>

<template>
  <div class="pane">
    <div class="pane-body">
      <span class="pane-title">{{ t('environments.new') }}</span>

      <div class="pair">
        <label class="field">
          <span class="field-label">{{ t('environments.nameLabel') }}</span>
          <input
            v-model="name"
            class="input"
            maxlength="40"
            spellcheck="false"
            :placeholder="t('environments.namePlaceholder')"
            @keydown.enter="create"
          />
        </label>

        <div class="field tight">
          <span class="field-label">{{ t('environments.colorLabel') }}</span>
          <ColorSwatches
            :value="color"
            :colors="ENVIRONMENT_COLORS"
            :tints="ENVIRONMENT_TINTS"
            :labels="colorLabels"
            @pick="color = $event"
          />
        </div>
      </div>

      <div class="field">
        <span class="field-label">{{ t('environments.startFrom') }}</span>
        <Segmented class="bases" :options="bases" :value="base" @pick="base = $event" />
      </div>

      <div class="field">
        <span class="field-label">{{ t('environments.variablesLabel') }}</span>
        <div class="card">
          <div class="card-note">{{ note }}</div>
          <div class="card-row">
            <span class="card-text">{{ t('environments.bringKeys') }}</span>
            <button type="button" class="card-button" @click="emit('import')">
              {{ t('environments.importEnv') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="foot">
      <span class="notice" :class="{ bad: !!notice }">{{ notice }}</span>
      <span class="head-spacer"></span>
      <button type="button" class="pane-cancel" @click="cancel">{{ t('common.cancel') }}</button>
      <button type="button" class="pane-action" @click="create">
        {{ t('environments.createAction') }}
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

.pane-title {
  @apply text-[15px] font-semibold;
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

/* The field the form opens on, drawn focused: the drawing shows it with the accent and the ring, and
   a name is what the form is waiting for. */
.input {
  @apply w-full h-[34px] px-[11px] rounded-lg text-[13.5px] text-text outline-none;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.input::placeholder {
  @apply text-text-tertiary;
}

.bases {
  max-width: 300px;
}

.card {
  @apply border border-border rounded-[10px] overflow-hidden;
}

.card-note {
  @apply flex items-center h-[42px] px-3 text-[13px] text-text-secondary;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.card-row {
  @apply flex items-center gap-2.5 h-11 px-3;
}

.card-text {
  @apply flex-1 min-w-0 text-[13px] text-text-tertiary;
}

.card-button {
  @apply flex-none h-[30px] px-3 rounded-[7px] cursor-pointer text-[12.5px] font-medium text-text;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.card-button:hover {
  @apply bg-bg-hover;
}

.foot .notice {
  @apply text-[12.5px] text-text-tertiary;
}

.notice.bad {
  @apply text-red;
}


/* The foot is pinned to the leaf and no longer scrolls away with the form. It is tall enough that
   the button in it has air above and below rather than sitting on the border, and its height is what
   keeps it level with the rail's own: that foot is 10px of padding around a 28px button, which puts
   the button's centre 24px above the bottom, and this one centres whatever stands in it. */
.foot {
  gap: 10px;
  @apply flex-none flex items-center h-12 px-5;
  border-top: 1px solid var(--border);
}


.pane-cancel {
  @apply h-[34px] px-3.5 rounded-lg cursor-pointer text-[13.5px] font-medium text-text;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.pane-cancel:hover {
  @apply bg-bg-hover;
}

.pane-action {
  @apply h-[34px] px-4 rounded-lg border-0 cursor-pointer text-accent-text text-[13.5px] font-semibold;
  font-family: inherit;
  background: var(--accent);
}

.pane-action:hover {
  @apply brightness-110;
}
</style>
