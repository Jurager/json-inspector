<script setup lang="ts">
import { ref } from 'vue'
import Icon from '../ui/Icon.vue'
import type { Auth, Field, Scheme } from '../../../bindings/json-inspector/internal/domain'
import { useMessages } from '../../i18n'

// The fields of a scheme, drawn from what Go said they are. There is no template per scheme and no
// branch on the scheme's name: a scheme added on that side is drawn here the day it is added.
//
// The component is controlled — the answers come in and the edits go out — because the two places
// it is used write differently: the command line writes every keystroke, and a level of a tree saves
// the whole tree, so it holds the answers itself and saves them when the field is left.
const props = defineProps<{
  scheme: Scheme | null
  auth: Auth
  // The popover is small and a collection's tab has room; the design gives each its own scale.
  scale?: 'compact' | 'roomy'
}>()

const emit = defineEmits<{
  update: [key: string, value: string]
  commit: []
}>()

const { t } = useMessages()

// A secret is hidden until somebody asks to look at it. The choice is per field and per view: it is
// not an answer about the request and nothing writes it down.
const revealed = ref<Record<string, boolean>>({})

// A field with no answer yet is drawn at what the scheme starts it on, so a select shows the choice
// it is actually on rather than a blank.
function valueOf(field: Field): string {
  const given = props.auth.fields?.[field.key]
  if (given !== undefined && given !== null) return given
  return props.scheme?.fields?.find((f) => f.key === field.key)?.default ?? ''
}

// Which fields the current answers bring with them. A grant type that asks for a login has one and a
// grant that does not has nowhere to put it; the scheme says which is which, and the window draws
// what it is handed rather than working any of it out.
const fields = () => (props.scheme?.fields ?? []).filter(shown)

function shown(field: Field): boolean {
  if (!field.when) return true
  const held = props.auth.fields?.[field.when.key]
  const answer = held !== undefined && held !== null ? held : (defaultOf(field.when.key) ?? '')
  return (field.when.values ?? []).includes(answer)
}

function defaultOf(key: string): string {
  return props.scheme?.fields?.find((f) => f.key === key)?.default ?? ''
}

function onInput(field: Field, event: Event) {
  emit('update', field.key, (event.target as HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement).value)
}
</script>

<template>
  <div class="fields" :class="scale ?? 'compact'">
    <label v-for="field in fields()" :key="field.key" class="field" :class="{ full: field.full }">
      <span class="label">{{ t(field.label) }}</span>

      <div v-if="field.kind === 'password'" class="veiled">
        <input
          :type="revealed[field.key] ? 'text' : 'password'"
          :value="valueOf(field)"
          :placeholder="field.placeholder ? t(field.placeholder) : undefined"
          :class="{ mono: field.mono }"
          spellcheck="false"
          autocomplete="off"
          @input="onInput(field, $event)"
          @blur="emit('commit')"
          @keydown.enter.prevent="emit('commit')"
        />
        <button
          type="button"
          class="eye"
          :aria-label="t('request.auth.reveal')"
          @click="revealed[field.key] = !revealed[field.key]"
        >
          <Icon name="eye" :size="13" />
        </button>
      </div>

      <textarea
        v-else-if="field.kind === 'textarea'"
        :value="valueOf(field)"
        :placeholder="field.placeholder ? t(field.placeholder) : undefined"
        :style="{ height: field.height ? `${field.height}px` : undefined }"
        spellcheck="false"
        @input="onInput(field, $event)"
        @blur="emit('commit')"
        @keydown.enter.meta.prevent="emit('commit')"
      />

      <select
        v-else-if="field.kind === 'select'"
        :value="valueOf(field)"
        @change="onInput(field, $event); emit('commit')"
      >
        <option v-for="option in field.options ?? []" :key="option.value" :value="option.value">
          {{ t(option.label) }}
        </option>
      </select>

      <input
        v-else
        :type="field.kind === 'number' ? 'number' : 'text'"
        :value="valueOf(field)"
        :placeholder="field.placeholder ? t(field.placeholder) : undefined"
        :class="{ mono: field.mono }"
        spellcheck="false"
        autocomplete="off"
        @input="onInput(field, $event)"
        @blur="emit('commit')"
        @keydown.enter.prevent="emit('commit')"
      />

      <span v-if="field.hint" class="hint">{{ t(field.hint) }}</span>
    </label>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* Two columns with the full-width fields spanning both, which is how the design draws a Basic
   credential and an API key: the pair sits on one line and the question under it on its own. */
.fields {
  @apply grid grid-cols-2 items-start;
}

.field {
  @apply flex flex-col min-w-0;
}

.field.full {
  @apply col-span-2;
}

.fields.compact {
  gap: 8px;
}

.fields.compact .field {
  gap: 4px;
}

.fields.roomy {
  gap: 10px;
}

.fields.roomy .field {
  gap: 5px;
}

.label {
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 500;
}

.roomy .label {
  color: var(--text);
  font-size: 12px;
}

.fields input,
.fields textarea,
.fields select {
  @apply w-full border bg-bg-panel text-text box-border;
  font: inherit;
  outline: none;
}

.fields input,
.fields select {
  height: 28px;
  padding: 0 8px;
  border-radius: 6px;
  border-color: var(--border);
  font-size: 11.5px;
}

.fields textarea {
  min-height: 64px;
  padding: 6px 8px;
  border-radius: 6px;
  border-color: var(--border);
  font-size: 11px;
  line-height: 1.5;
  resize: none;
}

.fields select {
  cursor: pointer;
}

.roomy input,
.roomy select {
  height: 34px;
  padding: 0 10px;
  border-radius: 8px;
  border-color: var(--border-strong);
  font-size: 12.5px;
}

.roomy textarea {
  padding: 8px 10px;
  border-radius: 8px;
  border-color: var(--border-strong);
  font-size: 12px;
}

.fields input:focus,
.fields textarea:focus,
.fields select:focus {
  border-color: var(--accent);
}

.fields .mono {
  font-family: var(--mono);
}

.fields input::placeholder,
.fields textarea::placeholder {
  color: var(--text-tertiary);
}

.veiled {
  @apply relative flex min-w-0;
}

.veiled input {
  padding-right: 28px;
}

/* Edge draws its own reveal control inside every password field. It is not themed — on a dark field
   it comes out black and overflows the rounded corner — and it sits a few pixels from the one the
   design draws, so a password field showed two eyes. The design has one, and it is the one below:
   the same on every platform instead of only on the one this app happens to run through. */
.veiled input[type='password']::-ms-reveal,
.veiled input[type='password']::-ms-clear {
  display: none;
}

.eye {
  @apply absolute flex items-center justify-center border-none bg-transparent cursor-pointer text-text-tertiary;
  right: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 20px;
  height: 20px;
  border-radius: 5px;
  padding: 0;
}

.eye:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.hint {
  color: var(--text-tertiary);
  font-size: 10.5px;
  line-height: 1.4;
}
</style>
