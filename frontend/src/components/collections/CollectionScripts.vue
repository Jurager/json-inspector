<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { FLUSH_MS, useCollectionsStore } from '../../stores/collections'

const store = useCollectionsStore()

// The code of the selected level, held here while it is typed. What comes back from Go is applied
// only to an editor nobody is typing in: the answer can be older than the last keystroke.
const pre = ref('')
const post = ref('')
const dirty = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null

// What the level would run for one half if its own editor stayed empty. The editor shows it greyed
// out, because an empty box beside a chain of scripts is not a box that runs nothing.
function inherited(scope: 'pre' | 'post'): string {
  return store.inheritedScript(scope)?.text ?? ''
}

watch(
  [() => store.scriptsFor, () => store.scripts],
  () => {
    if (store.scriptsFor !== store.selectedId || dirty.value) return
    pre.value = store.scripts?.pre ?? ''
    post.value = store.scripts?.post ?? ''
  },
  { immediate: true }
)

// Opening the level is what reads its code: the tab is drawn for whatever is selected, and a level
// that changes under an open tab is a level whose editors have to follow.
watch(
  () => store.selectedId,
  (id) => {
    dirty.value = false
    void store.loadScripts(id)
  },
  { immediate: true }
)

// Typing is not a save gesture in this window: the code is written the way the command line's address
// is — a pause after the last keystroke, and again when the tab or the window is left.
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
  await store.saveScripts(pre.value, post.value)
}

onBeforeUnmount(() => void save())
</script>

<template>
  <div class="scripts">
    <label class="field">
      <span class="label">Pre-request script</span>
      <textarea
        v-model="pre"
        class="code"
        spellcheck="false"
        :placeholder="inherited('pre')"
        @input="onInput"
        @blur="save"
      />
    </label>

    <label class="field">
      <span class="label">Post-response script</span>
      <textarea
        v-model="post"
        class="code"
        spellcheck="false"
        :placeholder="inherited('post')"
        @input="onInput"
        @blur="save"
      />
    </label>

    <div class="note">
      <svg
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
      <span class="note-text">
        Выполняются перед каждым запросом коллекции и после каждого ответа — до собственных скриптов
        запроса, если они заданы.
      </span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.scripts {
  @apply flex flex-col gap-[18px] max-w-[620px] px-6 py-6;
}

.field {
  @apply flex flex-col gap-1.5;
}

.label {
  @apply text-[13px] font-semibold;
}

/* The size and the inset of the design's code box; a textarea rather than a div because this is the
   one place in the window where the user writes code. */
.code {
  @apply h-[88px] resize-y rounded-lg border border-border-strong bg-bg-inset
         px-[11px] py-[9px] font-mono text-[12px] leading-normal text-text;
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

.note-icon {
  @apply flex-none mt-px text-accent;
}

.note-text {
  @apply text-[12px] leading-normal;
}
</style>
