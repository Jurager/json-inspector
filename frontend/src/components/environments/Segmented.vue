<script setup lang="ts">
// A row of exclusive choices: what a new environment starts from, whether this one is editable, and
// what a workspace is meant to be. The shape is the window's own segmented control — the auth switch
// draws it in a request's sheet — and it is written once here because the panes ask for the same
// control with different words in it.
//
// An option may be drawn and not offered: `soon` is a choice the design shows so that the answer it
// will have is visible, and `soonHint` is what says so on hover. It never emits, and it is disabled
// on its own, so a row can hold one live choice beside one that is still coming.
defineProps<{
  options: { value: string; label: string; soon?: boolean }[]
  value: string
  soonHint?: string
  disabled?: boolean
}>()

const emit = defineEmits<{ (e: 'pick', value: string): void }>()
</script>

<template>
  <div class="segment" :class="{ disabled }">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="seg"
      :class="{ active: option.value === value, soon: option.soon }"
      :disabled="disabled || option.soon"
      :title="option.soon ? soonHint : undefined"
      @click="emit('pick', option.value)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.segment {
  @apply flex gap-0.5 p-0.5 rounded-lg bg-bg-inset;
}

.seg {
  @apply flex-1 min-w-0 text-center border-none rounded-md cursor-pointer
         bg-transparent text-text-secondary;
  font: inherit;
  font-size: 12.5px;
  font-weight: 500;
  height: 28px;
  padding: 0;
  /* A segment is one line by construction: a name that wrapped would make the row two rows tall and
     shove what stands under it out of place. A long environment's name ellipsizes instead. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
}

/* The chosen segment is the panel's own colour lifted off the groove it sits in. */
.seg.active {
  @apply bg-bg-panel text-text;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.segment.disabled {
  @apply opacity-50;
}

.seg:disabled {
  @apply cursor-default;
}

/* A choice that is drawn and not offered: it keeps its place in the row so the answer it will have is
   visible, and it is dimmed rather than hidden. */
.seg.soon {
  @apply text-text-tertiary;
}
</style>
