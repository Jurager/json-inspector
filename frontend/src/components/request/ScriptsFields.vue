<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { FLUSH_MS } from '../../stores/collections'
import type { RequestSource } from '../../lib/requestSource'

// The two boxes of code a request can run around itself, wherever it is asked for: in the header of a
// collection and in the chip of the card. The code is Go's, the typing is the window's, and the pause
// before writing is the same one the address in the command line gets.
const props = withDefaults(
  defineProps<{ source: RequestSource; note: string; compact?: boolean }>(),
  { compact: false }
)

const pre = ref('')
const post = ref('')
const dirty = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

// What would run for one half while this level's own box stays empty. Greyed out in the box, because an
// empty box beside a chain of code is not a box that does nothing.
function inherited(scope: 'pre' | 'post'): string {
  return props.source.inheritedScript(scope)?.text ?? ''
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
    if (level) void props.source.loadScripts()
  },
  { immediate: true }
)

// Typing is not a save gesture in this window: the code is written the way the command line's address
// is — a pause after the last keystroke, and again when the box or the window is left.
function onInput() {
  dirty.value = true
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => void save(), FLUSH_MS)
}

async function save() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  if (!dirty.value) return
  // Cleared before the write and not after it: a keystroke that arrives while the save travels is an
  // edit the next pause has to write again.
  dirty.value = false
  await props.source.saveScripts(pre.value, post.value)
}

onBeforeUnmount(() => void save())
</script>

<template>
  <div class="fields" :class="{ compact: props.compact }">
    <label class="field">
      <span class="label">{{ props.compact ? 'Pre-request' : 'Pre-request script' }}</span>
      <textarea
        v-model="pre"
        class="code"
        spellcheck="false"
        :placeholder="inherited('pre') || (props.compact ? '// выполняется до отправки' : '')"
        @input="onInput"
        @blur="save"
      />
    </label>

    <label class="field">
      <span class="label">{{ props.compact ? 'Post-response' : 'Post-response script' }}</span>
      <textarea
        v-model="post"
        class="code"
        spellcheck="false"
        :placeholder="inherited('post') || (props.compact ? '// проверки ответа' : '')"
        @input="onInput"
        @blur="save"
      />
    </label>

    <div class="note" :class="{ plain: props.compact }">
      <svg
        v-if="!props.compact"
        class="note-icon"
        width="13"
        height="13"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v5M12 16h.01" stroke-linecap="round" />
      </svg>
      <span class="note-text">{{ props.note }}</span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.fields {
  @apply flex flex-col gap-[18px];
}

.fields.compact {
  @apply gap-2;
}

.field {
  @apply flex flex-col gap-1.5;
}

.label {
  @apply text-[13px] font-semibold;
}

.fields.compact .label {
  @apply text-[11.5px] font-medium text-text-secondary;
}

/* The size and the inset of the design's code box; a textarea rather than a div because this is the
   one place in the window where the user writes code. */
.code {
  @apply h-[88px] resize-y rounded-lg border border-border-strong bg-bg-inset
         px-[11px] py-[9px] font-mono text-[12px] leading-normal text-text;
}

.fields.compact .code {
  @apply h-[72px] rounded-[7px] px-2.5 py-2 text-[11.5px];
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
  @apply bg-transparent p-0 px-1 text-[11px] text-text-tertiary;
}

.note-icon {
  @apply flex-none mt-px text-accent;
}

.note-text {
  @apply text-[12px] leading-normal;
}

.note.plain .note-text {
  @apply text-[11px];
}
</style>
