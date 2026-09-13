<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import Tooltip from '../ui/tooltip/Tooltip.vue'
import { Checkbox } from '../ui/checkbox'
import { formatBody } from '../../lib/bodyFormat'
import { useMessages } from '../../i18n'
import type { RequestSource } from '../../lib/requestSource'
import { BodyKind, RowKind, type FormRow } from '../../../bindings/json-inspector/internal/domain'

// The Body chip's popover. It is the one chip whose subject is not a list but a format: the five
// segments pick what the request carries, and each of them draws the editor that suits it.
//
// Five branches in one file rather than five components: they share every style below, they are
// never used apart, and the popover already draws its chips with a five-way `v-if`.
const props = defineProps<{ source: RequestSource }>()

const { t } = useMessages()

const store = props.source

// The five formats, in the order the handoff draws them.
const KINDS: { kind: BodyKind; label: string }[] = [
  { kind: BodyKind.BodyJSON, label: 'JSON' },
  { kind: BodyKind.BodyXML, label: 'XML' },
  { kind: BodyKind.BodyRaw, label: 'Raw' },
  { kind: BodyKind.BodyForm, label: 'Form-Data' },
  { kind: BodyKind.BodyBinary, label: 'Binary' },
]

const kind = computed(() => store.bodyKind)

// The pill moves one segment (+ the 2px gap) per step, animated by a CSS transition on transform.
// This is the Auth chip's control with five segments instead of four: one pill under the group
// rather than a background on each button, so the eye follows the choice instead of watching it
// appear. -1 is a kind the list does not know, which parks the pill on the first segment.
const activeKindIndex = computed(() => {
  const at = KINDS.findIndex((option) => option.kind === store.bodyKind)
  return at < 0 ? 0 : at
})
const indicatorStyle = computed(() => ({
  transform: `translateX(calc(${activeKindIndex.value} * (100% + 2px)))`,
}))
const isText = computed(
  () => kind.value === BodyKind.BodyJSON || kind.value === BodyKind.BodyXML || kind.value === BodyKind.BodyRaw,
)
const hasToolbar = computed(() => kind.value === BodyKind.BodyJSON || kind.value === BodyKind.BodyXML)

const area = ref<HTMLTextAreaElement | null>(null)
const findText = ref('')

function onInput(e: Event) {
  store.setBody((e.target as HTMLTextAreaElement).value)
}

// Reformatting replaces the text with a re-typed version of itself, so it goes through the same
// buffer the typing does and is handed over on the same clock.
function format() {
  const next = formatBody(kind.value, store.body)
  if (next === store.body) return
  store.setBody(next)
  void store.flush()
}

// A find that reports nothing is a find that did nothing: the selection stays where the caret was,
// which is where the user will look for the text themselves.
function find() {
  const el = area.value
  const needle = findText.value
  if (!el || needle === '') return
  const at = el.value.indexOf(needle)
  if (at < 0) return
  el.focus()
  el.setSelectionRange(at, at + needle.length)
}

function patch(id: string, patch: Record<string, unknown>) {
  void store.patchRow(RowKind.RowForm, id, patch)
}

function toggle(row: FormRow, enabled: boolean) {
  void store.toggleRow(RowKind.RowForm, row.id, enabled)
}

async function pickFile(row: FormRow) {
  const path = await store.pickBodyFile()
  if (path) patch(row.id, { src: path, file: true })
}

async function pickBody() {
  const path = await store.pickBodyFile()
  if (path) void store.setBodyFile(path)
}

// The last path segment, for a chip that has room for a name and not for a path.
function baseName(path: string): string {
  const parts = path.split(/[\\/]/)
  return parts[parts.length - 1] || path
}

// Numbers take --tok-num, everything else --tok-str, the same as the parameters and headers rows:
// a form value is a value, and the design colours it as one.
function valueClass(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : 'str'
}
</script>

<template>
  <div class="body">
    <div class="kinds">
      <span class="kind-indicator" :style="indicatorStyle"></span>
      <button
        v-for="option in KINDS"
        :key="option.kind"
        class="kind"
        :class="{ active: store.bodyKind === option.kind }"
        @click="store.setBodyKind(option.kind)"
      >
        {{ option.label }}
      </button>
    </div>

    <template v-if="isText">
      <!--
        The editor is a plain textarea and not a syntax-highlighted one: the body can hold
        `{{tokens}}`, and this app draws those by painting a layer over a real input. A rich editor
        cannot take part in that overlay, and the design asks for no highlighting here anyway.
      -->
      <div v-if="hasToolbar" class="tools">
        <Button variant="outline" size="sm" @click="format">
          <Icon name="list" :size="12" />
          <span>{{ t('request.body.format') }}</span>
        </Button>
        <div class="tools-spacer"></div>
        <div class="find">
          <Icon name="search" :size="10" />
          <input
            v-model="findText"
            class="find-input"
            :placeholder="t('request.body.findInBody')"
            spellcheck="false"
            @keydown.enter.prevent="find"
          />
        </div>
      </div>
      <textarea
        ref="area"
        :value="store.body"
        class="editor"
        :class="{ brief: hasToolbar }"
        :placeholder="kind === BodyKind.BodyRaw ? t('request.body.rawPlaceholder') : ''"
        spellcheck="false"
        @input="onInput"
        @blur="store.flush()"
      ></textarea>
    </template>

    <template v-else-if="kind === BodyKind.BodyForm">
      <!-- The grid and its head are one block with a 2px gap, not two children of the popover with
           its 8px: the head belongs to the grid, and the handoff draws them tight together. -->
      <div class="form">
        <div class="grid grid-head">
          <span></span><span>{{ t('request.body.columnKey') }}</span><span>{{ t('request.body.columnValue') }}</span><span></span>
        </div>
        <div v-for="row in store.form" :key="row.id" class="grid form-row" :class="{ off: !row.enabled }">
          <Checkbox :model-value="row.enabled" @update:model-value="toggle(row, $event)" @click.stop />
          <input
            :value="row.name"
            class="cell mono"
            :placeholder="t('request.placeholderKey')"
            spellcheck="false"
            @input="patch(row.id, { name: ($event.target as HTMLInputElement).value })"
          />
          <div class="value-cell">
            <template v-if="row.file">
              <button class="file" @click="pickFile(row)">
                <Icon name="download" :size="10" />
                <span>{{ row.src ? baseName(row.src) : t('request.body.chooseFile') }}</span>
              </button>
              <Tooltip side="top">
                <template #trigger>
                  <button class="icon lit" @click="patch(row.id, { file: false })">
                    <Icon name="link" :size="12" />
                  </button>
                </template>
                {{ t('request.body.makeTextField') }}
              </Tooltip>
            </template>
            <template v-else>
              <input
                :value="row.value"
                class="cell mono"
                :class="valueClass(row.value)"
                :placeholder="t('request.placeholderValue')"
                spellcheck="false"
                @input="patch(row.id, { value: ($event.target as HTMLInputElement).value })"
              />
              <Tooltip side="top">
                <template #trigger>
                  <button class="icon" @click="patch(row.id, { file: true })">
                    <Icon name="link" :size="12" />
                  </button>
                </template>
                {{ t('request.body.makeFileField') }}
              </Tooltip>
            </template>
          </div>
          <IconButton variant="danger" size="sm" :hint="t('common.delete')" @click.stop="store.removeRow(RowKind.RowForm, row.id)">
            <Icon name="trash" :size="13" />
          </IconButton>
        </div>
        <div class="popover-foot">
          <Button variant="ghost" size="sm" @click="store.addRow(RowKind.RowForm)">
            <Icon name="plus" :size="16" />
            <span>{{ t('request.addField') }}</span>
          </Button>
          <span class="foot-hint">{{ t('request.formFoot') }}</span>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="drop">
        <Icon name="download" :size="20" />
        <span class="drop-name">{{ store.bodyFile ? baseName(store.bodyFile) : t('request.body.noFileChosen') }}</span>
        <Button size="sm" @click="pickBody">{{ t('request.body.chooseFile') }}</Button>
      </div>
    </template>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.body {
  @apply flex flex-col gap-2;
}

/* The Auth chip's segmented control, five-wide instead of four: the pill is one element under the
   buttons and slides, rather than each button painting itself. The handoff draws this control a
   little tighter than Auth's and the two do not have to agree — the app's own does. */
.kinds {
  @apply relative flex gap-0.5 p-0.5 bg-bg-inset;
  border-radius: 7px;
}

.kind-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 bg-bg-panel;
  border-radius: 5px;
  /* Four 2px gaps and the track's own 2px of padding: (100% - 4px - 8px) / 5. */
  width: calc((100% - 12px) / 5);
  transition: transform 0.18s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.18), 0 0 0 0.5px var(--border-strong);
}

.kind {
  @apply relative flex-1 text-center text-xs py-1 border-none bg-transparent text-text-secondary cursor-pointer;
  border-radius: 5px;
  transition: color 0.18s ease;
}

.kind.active {
  @apply font-medium text-text;
}

.tools {
  @apply flex items-center gap-1;
}

.tools-spacer {
  @apply flex-1;
}

.find {
  @apply flex items-center gap-1;
  height: 22px;
  padding: 0 8px;
  width: 130px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--bg-panel);
  color: var(--text-tertiary);
}

.find-input {
  @apply flex-1 min-w-0 border-none outline-none bg-transparent;
  font-size: 11px;
  color: var(--text);
}

.find-input::placeholder {
  color: var(--text-tertiary);
}

.editor {
  @apply w-full outline-none select-text resize-y;
  font-family: var(--mono);
  border-radius: 8px;
  padding: 8px 10px;
  background: var(--bg-inset);
  border: 1px solid var(--border);
  color: var(--text);
  height: 132px;
  font-size: 11.5px;
}

/* The two with a toolbar are shorter, because the toolbar takes the room the design gives them. */
.editor.brief {
  height: 116px;
  line-height: 1.6;
}

.editor:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.editor::placeholder {
  color: var(--text-tertiary);
}

.form {
  @apply flex flex-col gap-0.5;
}

.grid {
  @apply grid gap-1.5 px-1;
  grid-template-columns: 20px 150px 1fr 22px;
}

/* The head is a label row and not a control row: it takes the grid's columns but not its vertical
   centring, and the 4px under it is what separates it from the first row. */
.grid-head {
  padding-bottom: 4px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
}

.form-row {
  @apply items-center;
  padding-top: 3px;
  padding-bottom: 3px;
}

.form-row.off {
  @apply opacity-55;
}

.cell {
  @apply min-w-0 w-full bg-transparent border-0 outline-none p-0 rounded-sm;
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--text);
}

.cell:focus {
  background: var(--bg-inset);
}

/* A form value is a value like any other, so it is coloured like one: a number takes the number
   token, everything else the string token. */
.cell.str {
  color: var(--tok-str);
}

.cell.num {
  color: var(--tok-num);
}

.value-cell {
  @apply relative flex items-center gap-1.5 min-w-0;
}

/* Dashed because the file is not picked yet — the same "a thing that is not there yet" the body and
   scripts chips are drawn with — and small, because it shares a row with an input and a button. */
.file {
  @apply flex-1 flex items-center cursor-pointer min-w-0;
  gap: 5px;
  height: 22px;
  padding: 0 7px;
  border-radius: 5px;
  border: 1px dashed var(--border-strong);
  background: transparent;
  color: var(--text-tertiary);
  font-size: 10.5px;
}

.file:hover {
  border-color: var(--accent);
  color: var(--text);
}

.file span {
  @apply overflow-hidden text-ellipsis whitespace-nowrap;
}

/* The paperclip is the only control in this grid that is not one of the app's own: it has a second
   state — the row is a file field rather than a text one — that no IconButton variant carries. It is
   drawn at that button's small size, 20px, so that it and the trash beside it are the same box: the
   grid's last column is 22, and anything larger would push the row open.

   The two icons carry the same ink at those sizes, which is not automatic: lucide's `link` fills its
   viewBox (10.3px at 12) and its `trash` nearly so (10.6px at 13), but a plain ✕ fills half of one,
   which is why the delete is a bin here and not a cross. */
.icon {
  @apply flex-none inline-flex items-center justify-center cursor-pointer border-none;
  width: 20px;
  height: 20px;
  padding: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-tertiary);
  line-height: 0;
}

.icon:hover {
  background: var(--bg-hover);
  color: var(--text);
}

/* The lit paperclip: this row is a file field rather than a text one. */
.icon.lit {
  background: var(--accent-soft);
  color: var(--accent);
}

.icon.lit:hover {
  background: color-mix(in srgb, var(--accent) 22%, transparent);
  color: var(--accent);
}

.drop {
  @apply flex flex-col items-center justify-center gap-2;
  height: 132px;
  border-radius: 8px;
  border: 1px dashed var(--border-strong);
  background: var(--bg-inset);
  color: var(--text-tertiary);
}

.drop-name {
  font-size: 12px;
  color: var(--text-secondary);
  max-width: 90%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
