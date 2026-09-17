<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { FLUSH_MS } from '../../stores/collections'
import { useMessages } from '../../i18n'
import type { RequestSource } from '../../lib/requestSource'

// The two halves of code a level can run around its requests, wherever they are asked for: in the
// sheet of a collection and in the chip of a card. Two shapes over one source — the chip shows both
// halves at once in the narrow room it has, and the sheet shows one at a time behind a switch,
// because the handoff draws it that way and code wants the width.
//
// They also write differently, and for the same reason the auth sheet does: the chip saves with the
// pause the command line's address gets, and the sheet holds what is in the box until its footer
// says Save — a box that wrote on every pause would leave the footer's Cancel nothing to cancel.
const props = withDefaults(
  defineProps<{ source: RequestSource; note: string; compact?: boolean }>(),
  { compact: false }
)

const { t } = useMessages()

const pre = ref('')
const post = ref('')
// A half can be switched off without being emptied: the code stays, and what changes is whether it
// runs. The two flags travel with the code, so the level above cannot hand down a script it has
// switched off.
const off = ref<{ pre: boolean; post: boolean }>({ pre: false, post: false })
const dirty = ref(false)
// Which half the sheet is showing. The chip draws both, so the switch is the sheet's own.
const scope = ref<'pre' | 'post'>('pre')
let timer: ReturnType<typeof setTimeout> | null = null

const code = computed({
  get: () => (scope.value === 'pre' ? pre.value : post.value),
  set: (text: string) => {
    if (scope.value === 'pre') pre.value = text
    else post.value = text
  },
})

// The box says how much code is in it: a script is a few lines, and the count is what tells a reader
// whether the box they are looking at is the one they wrote.
const meta = computed(() =>
  t('collections.scriptMeta', { n: code.value.split('\n').filter((line) => line.trim()).length })
)

// What the switch by the box says: the half the editor is showing, on or off. Switching it is an edit
// like typing is — it travels with the footer's Save, and Cancel puts it back.
const halfOff = computed(() => off.value[scope.value])

function toggleHalf() {
  off.value = { ...off.value, [scope.value]: !off.value[scope.value] }
  dirty.value = true
}

// What would run for one half while this level's own box stays empty. Greyed out in the box, because an
// empty box beside a chain of code is not a box that does nothing.
function inherited(which: 'pre' | 'post'): string {
  return props.source.inheritedScript(which)?.text ?? ''
}

// What the boxes show: the code of the level the window is on. Reading it is the level's own business —
// a collection card reads the node it opened, the command line reads its draft — so the level is asked
// for and the answer is applied only when it is about that level and nobody is typing in it.
watch(
  [() => props.source.scriptsLevel, () => props.source.scripts],
  ([level]) => {
    if (!level || props.source.scriptsFor !== level || dirty.value) return
    pre.value = props.source.scripts?.pre ?? ''
    post.value = props.source.scripts?.post ?? ''
    off.value = {
      pre: props.source.scripts?.preOff ?? false,
      post: props.source.scripts?.postOff ?? false,
    }
  },
  { immediate: true }
)

watch(
  () => props.source.scriptsLevel,
  (level) => {
    dirty.value = false
    // The level changed: what is in the boxes belongs to the one that was open before.
    pre.value = ''
    post.value = ''
    off.value = { pre: false, post: false }
    if (level) void props.source.loadScripts()
  },
  { immediate: true }
)

function onInput() {
  dirty.value = true
  if (!props.compact) return
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => void commit(), FLUSH_MS)
}

async function commit() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  if (!dirty.value) return
  // Cleared before the write and not after it: a keystroke that arrives while the save travels is an
  // edit the next pause has to write again.
  dirty.value = false
  await props.source.saveScripts(pre.value, post.value, off.value)
}

// What Cancel does: the box goes back to what the level actually holds, and nothing travels.
function discard() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  dirty.value = false
  pre.value = props.source.scripts?.pre ?? ''
  post.value = props.source.scripts?.post ?? ''
  off.value = {
    pre: props.source.scripts?.preOff ?? false,
    post: props.source.scripts?.postOff ?? false,
  }
}

defineExpose({ commit, discard })

onBeforeUnmount(() => void commit())
</script>

<template>
  <div v-if="compact" class="fields compact">
    <label class="field">
      <span class="label">Pre-request</span>
      <textarea
        v-model="pre"
        class="code"
        spellcheck="false"
        :placeholder="inherited('pre') || t('request.body.prePlaceholder')"
        @input="onInput"
        @blur="commit"
      />
    </label>

    <label class="field">
      <span class="label">Post-response</span>
      <textarea
        v-model="post"
        class="code"
        spellcheck="false"
        :placeholder="inherited('post') || t('request.body.postPlaceholder')"
        @input="onInput"
        @blur="commit"
      />
    </label>

    <div class="note plain">
      <span class="note-text">{{ props.note }}</span>
    </div>
  </div>

  <!-- The sheet's shape: one half at a time behind the switch the handoff draws, and the box as tall
       as a short script. -->
  <div v-else class="sheet-fields">
    <div class="switch">
      <button
        v-for="half in (['pre', 'post'] as const)"
        :key="half"
        type="button"
        class="seg"
        :class="{ active: scope === half }"
        @click="scope = half"
      >
        {{ half === 'pre' ? t('collections.scriptPre') : t('collections.scriptPost') }}
      </button>
    </div>

    <!-- Code that is switched off is drawn the way the sheet draws anything that is not this level's
         own: tertiary ink on a veiled surface. The code stays where it is; what changes is whether it
         runs. -->
    <textarea
      v-model="code"
      class="editor"
      :class="{ off: halfOff }"
      spellcheck="false"
      :placeholder="inherited(scope)"
      @input="onInput"
    />

    <div class="meta">
      <button type="button" class="toggle" :class="{ off: halfOff }" @click="toggleHalf">
        {{ halfOff ? t('collections.scriptEnable') : t('collections.scriptDisable') }}
      </button>
      <span class="meta-text">{{ meta }}</span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.fields {
  @apply flex flex-col gap-[18px];
}

/* The popover's own rhythm carried inside: its gap is 12, and each box's label stands 7px above it. */
.fields.compact {
  @apply gap-3;
}

.field {
  @apply flex flex-col gap-1.5;
}

.label {
  @apply text-[13px] font-semibold;
}

.fields.compact .label {
  @apply text-[12px] font-medium text-text-secondary;
}

.fields.compact .field {
  gap: 7px;
}

/* The size and the inset of the design's code box; a textarea rather than a div because this is the
   one place in the window where the user writes code. */
.code {
  @apply h-[88px] resize-y rounded-lg border border-border-strong bg-bg-inset
         px-[11px] py-[9px] font-mono text-[12px] leading-normal text-text;
}

.fields.compact .code {
  @apply h-[84px] rounded-[9px] px-3 py-2.5 text-[13px] leading-[1.6];
}

.code::placeholder {
  @apply text-text-tertiary;
}

.code:focus {
  @apply outline-none border-accent;
}

.note {
  @apply flex items-start gap-2 px-3 py-2.5 rounded-lg bg-accent-soft;
}

/* The chip's note is a line under the boxes, not a callout: it is the same sentence in less room. */
.note.plain {
  @apply bg-transparent p-0 text-[12.5px] text-text-tertiary;
}

.note-icon {
  @apply flex-none mt-px text-accent;
}

.note-text {
  @apply text-[12px] leading-normal;
}

.note.plain .note-text {
  @apply text-[12.5px];
}

/* ---- the sheet's half of this component ---- */

.sheet-fields {
  @apply flex flex-col gap-3;
}

/* The switch of the drawing: a groove with the chosen half lifted out of it, the two halves of one
   script rather than two scripts. */
.switch {
  @apply flex gap-0.5 p-0.5 rounded-lg bg-bg-inset;
}

.seg {
  @apply flex-1 min-w-0 h-7 border-0 rounded-md bg-transparent cursor-pointer
         text-[12.5px] font-medium text-text-secondary;
  font-family: inherit;
  transition: background 0.15s ease, color 0.15s ease;
}

.seg.active {
  @apply bg-bg-panel text-text font-semibold;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.editor {
  @apply min-h-[148px] resize-y rounded-[9px] border border-border-strong bg-bg-inset
         px-3.5 py-3 font-mono text-[12.5px] text-text;
  line-height: 1.6;
}

.editor::placeholder {
  @apply text-text-tertiary;
}

.editor:focus {
  @apply outline-none border-accent;
}

.editor.off {
  @apply bg-bg-hover text-text-tertiary;
}

/* The switch is the drawing's own: a small framed button that reads as a state rather than as an
   action — green while the half runs, and the plain surface when it does not. */
.toggle {
  @apply flex-none h-[30px] px-[11px] rounded-[7px] border text-[12.5px] font-medium cursor-pointer
         bg-green-soft border-green-soft;
  font-family: inherit;
  color: var(--green-text);
}

.toggle:hover {
  @apply brightness-105;
}

.toggle.off {
  @apply bg-bg-inset border-border text-text-secondary;
}

.meta {
  @apply flex items-center gap-2.5;
}

.meta-text {
  @apply text-[12.5px] text-text-tertiary;
}
</style>
