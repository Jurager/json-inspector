<script setup lang="ts">
import { ref } from 'vue'
import Icon from '../ui/Icon.vue'
import VarHighlight from '../ui/VarHighlight.vue'
import { useMessages } from '../../i18n'
import { parseTokens } from '../../lib/vars'
import type { Auth, Field, Scheme } from '../../../bindings/json-inspector/internal/domain'

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

// A credential a level writes down is a `{{token}}` far more often than it is the secret itself: the
// value lives in the environment, and the level only says which of them it sends. So a field draws
// its tokens the way the command line draws them — the pill is painted over the input, which is still
// the thing being typed into.
function tokensIn(value: string): boolean {
  return value.trim() !== '' && parseTokens(value).length > 0
}

// Whether the pills go over this value: a textarea only while it holds one line, because the layer
// has no way of knowing where the box would have wrapped it.
function covered(value: string): boolean {
  return !value.includes('\n') && tokensIn(value)
}

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

      <!-- The pills are laid over the box that is being typed into, so the two have to agree on where
           the text starts: a long value is a textarea, and its pills are drawn only while it holds a
           single line, which is the case a reference is. -->
      <div v-else-if="field.kind === 'textarea'" class="covered" :class="{ mono: field.mono }">
        <textarea
          :value="valueOf(field)"
          :placeholder="field.placeholder ? t(field.placeholder) : undefined"
          :class="{ veiled: covered(valueOf(field)) }"
          :style="{ height: field.height ? `${field.height}px` : undefined }"
          spellcheck="false"
          @input="onInput(field, $event)"
          @blur="emit('commit')"
          @keydown.enter.meta.prevent="emit('commit')"
        />
        <VarHighlight v-if="covered(valueOf(field))" :value="valueOf(field)" multiline />
      </div>

      <select
        v-else-if="field.kind === 'select'"
        :value="valueOf(field)"
        @change="onInput(field, $event); emit('commit')"
      >
        <option v-for="option in field.options ?? []" :key="option.value" :value="option.value">
          {{ t(option.label) }}
        </option>
      </select>

      <div v-else class="covered" :class="{ mono: field.mono }">
        <input
          :type="field.kind === 'number' ? 'number' : 'text'"
          :value="valueOf(field)"
          :placeholder="field.placeholder ? t(field.placeholder) : undefined"
          :class="{ veiled: covered(valueOf(field)) }"
          spellcheck="false"
          autocomplete="off"
          @input="onInput(field, $event)"
          @blur="emit('commit')"
          @keydown.enter.prevent="emit('commit')"
        />
        <VarHighlight v-if="covered(valueOf(field))" :value="valueOf(field)" />
      </div>

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

/* Where the text of a field starts, for the layer that paints over it: the two have to agree, and it
   is the scale that decides. */
.fields.compact {
  --field-pad: 0 12px;
  gap: 12px;
}

.fields.roomy {
  --field-pad: 0 10px;
  gap: 10px;
}

.fields.compact .field {
  gap: 6px;
}

.fields.roomy .field {
  gap: 5px;
}

.label {
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
}

/* The sheet's own label: the handoff draws every label in it as the same small caps line the blocks
   above use, and the popover keeps the plainer one it was drawn with. */
.roomy .label {
  @apply text-text-tertiary;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

.fields input,
.fields textarea,
.fields select {
  @apply w-full border bg-bg-panel text-text box-border;
  font: inherit;
  outline: none;
}

/* A field and the layer of pills drawn over it are one box seen twice, so their padding and their
   type come from one rule each: two rules that agree today are two rules that drift apart. The
   height is the field's alone — the layer takes its box from `inset`, and a field may be as tall as
   its scheme said. */
.fields input,
.fields select {
  height: 34px;
}

.fields input,
.fields select,
.fields .highlight {
  padding: 0 12px;
  font-size: 13px;
}

.fields textarea,
.fields .highlight.area {
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.6;
}

.fields input,
.fields textarea,
.fields select {
  border-color: var(--border-strong);
  border-radius: 8px;
}

.fields textarea {
  resize: none;
}

.fields select {
  cursor: pointer;
}

/* The sheet's fields are the height of the one field row it draws: 34px, the same frame the popover
   gives them, in the sheet's own smaller type. */
.roomy input,
.roomy select {
  height: 34px;
}

/* The sheet's fields hold a credential, which is a value: the drawing gives every one of them the
   mono face and the same 13px the chip on them is drawn in. That they agree is not only a matter
   of taste — the pills stand where the characters are, and a pill in another face or another size
   would stand beside them.
   `.highlight.area` is spelled out because the compact rule above names it too, and a rule with one
   class more wins whatever the order: the layer over a box of code would otherwise keep the
   popover's size. */
.roomy input,
.roomy select,
.roomy textarea,
.roomy .highlight,
.roomy .highlight.area {
  font-family: var(--mono);
  font-size: 13px;
}

.roomy input,
.roomy select,
.roomy .highlight {
  padding: 0 10px;
}

.roomy textarea,
.roomy .highlight.area {
  padding: 8px 10px;
}

.roomy input,
.roomy textarea,
.roomy select {
  border-color: var(--border-strong);
  border-radius: 8px;
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

/* The box a field's pills are painted over: the field keeps the caret and the typing, and gives up
   the ink the layer draws instead. The face is the box's and not the field's, so that the field and
   the layer over it are drawn in the same one — a field declared mono would otherwise put its text
   in one face and its pills in another, and the last brace of a `{{token}}` would move the whole
   line sideways as the pills took over. */
.covered {
  @apply relative flex min-w-0;
}

.covered input.veiled,
.covered textarea.veiled {
  color: transparent;
  caret-color: var(--text);
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
  font-size: 11px;
  line-height: 1.4;
}
</style>
