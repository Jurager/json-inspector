<script setup lang="ts">
import { computed } from 'vue'
import VarToken from './VarToken.vue'
import { parseTokens, tokenSegments } from '../../lib/vars'

// A row's value: the editable input, and over it the same text with its `{{tokens}}` painted as
// pills. The input stays the control that keeps the caret and the typing — the layer above is
// transparent, is under no pointer, and is a second rendering of the same characters, so the two
// line up letter for letter.
//
// The layer is not decoration: a token written as plain text is a name nobody can read and nobody
// can check, and the pill carries what it comes to in its own tooltip.
const props = defineProps<{
  value: string
  placeholder?: string
}>()

const emit = defineEmits<{ (e: 'change', value: string): void }>()

const segments = computed(() => tokenSegments(props.value))
const hasTokens = computed(() => parseTokens(props.value).length > 0)

// Numbers take --tok-num, everything else --tok-str, mirroring the JSON tree's value highlighting.
const valueClass = computed(() => (/^-?\d+(\.\d+)?$/.test(props.value.trim()) ? 'num' : 'str'))

// The layer scrolls only when it is told to: the input scrolls on its own, and a pill left behind
// would name a token that is no longer under the caret.
function syncScroll(e: Event) {
  const input = e.target as HTMLInputElement
  const display = input.parentElement?.querySelector<HTMLElement>('.row-display')
  if (display) display.scrollLeft = input.scrollLeft
}
</script>

<template>
  <div class="row-cell">
    <input
      :value="value"
      class="row-input mono"
      :class="[valueClass, { 'row-input-veiled': hasTokens }]"
      :placeholder="placeholder"
      spellcheck="false"
      @input="emit('change', ($event.target as HTMLInputElement).value); syncScroll($event)"
      @scroll="syncScroll"
    />
    <span
      v-if="hasTokens"
      class="row-input row-display mono"
      :class="valueClass"
      aria-hidden="true"
    >
      <template v-for="(seg, si) in segments" :key="si">
        <VarToken v-if="seg.tokenName" :name="seg.tokenName" :text="seg.text" :offset="seg.start" />
        <span v-else>{{ seg.text }}</span>
      </template>
    </span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The cell the field and its layer share: the layer is positioned against it, so it is the box the
   two are aligned in. */
.row-cell {
  @apply relative flex min-w-0;
}

.row-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-[13px] p-0 rounded-sm;
  font-family: var(--mono);
  color: var(--text);
}

.row-input:focus {
  background: var(--bg-inset);
}

/* The field is left in place and made invisible rather than hidden: the caret has to stay in it, and
   an `opacity: 0` field would take the selection highlight with it. */
.row-input.row-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

.row-display {
  @apply absolute inset-0 flex items-center overflow-hidden pointer-events-none;
  white-space: pre;
}

.row-input.str {
  color: var(--tok-str);
}

.row-input.num {
  color: var(--tok-num);
}
</style>
