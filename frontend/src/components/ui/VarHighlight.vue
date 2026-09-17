<script setup lang="ts">
import { computed } from 'vue'
import VarToken from './VarToken.vue'
import { tokenSegments } from '../../lib/vars'

// The same text a field holds, with its `{{tokens}}` painted as pills. It is laid over the real
// input — which is the one that keeps the caret and the typing — so the two have to line up
// character for character: every segment here is a piece of the value, in its own order.
//
// What a token comes to is not written here: the pill carries it in its own tooltip, which is where
// a value made of other values belongs — a line of it under every field would be the sheet reading
// the environment out loud.
const props = withDefaults(defineProps<{ value: string; multiline?: boolean }>(), {
  multiline: false,
})

const segments = computed(() => tokenSegments(props.value))
</script>

<template>
  <span class="highlight" :class="{ area: multiline }" aria-hidden="true">
    <template v-for="(seg, si) in segments" :key="si">
      <VarToken v-if="seg.tokenName" :name="seg.tokenName" :text="seg.text" :offset="seg.start" />
      <span v-else>{{ seg.text }}</span>
    </template>
  </span>
</template>

<style scoped>
@reference "../../style.css";

/* The box of the field it covers, so a pill sits where the token's characters are: the padding, the
   type and the line height are the field's own, and they come from the same rule the field is drawn
   by — a layer styled on its own drifts a pixel off the text it is supposed to be. */
.highlight {
  @apply absolute inset-0 flex items-center overflow-hidden;
  padding: var(--field-pad, 0 12px);
  white-space: pre;
  pointer-events: none;
}

/* A box of code: the first line of the text starts under the top padding rather than in the middle
   of the box, and the layer has to start there too. The chip's own padding is given back here as a
   negative margin, for the same reason it is on the length: a chip laid on the first line would
   otherwise stand its characters three pixels below the line they belong to, and the token would
   drop as the closing braces turned it into a chip. In the middle of a one-line field there is
   nothing to give back — the chip is centred there, padding and all. */
.highlight.area {
  @apply items-start;
}

/* `:deep` because the chip is drawn inside a tooltip, so the attribute this file's styles are scoped
   by lands on the tooltip's root rather than on the chip itself. */
.highlight.area :deep(.var-token) {
  margin-block: -3px;
}
</style>
