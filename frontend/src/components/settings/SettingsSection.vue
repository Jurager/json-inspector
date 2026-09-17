<script setup lang="ts">
// A group of rows under a heading, in the drawing's clothes: an uppercase label, a card drawn around
// the rows by a hairline and nothing else, and a sentence under the card when the whole group needs
// one — a category that is not built says so here.
withDefaults(defineProps<{ label: string; note?: string }>(), { note: '' })
</script>

<template>
  <div class="section">
    <span class="label">{{ label }}</span>
    <div class="card"><slot /></div>
    <span v-if="note" class="note">{{ note }}</span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.section {
  @apply flex flex-col gap-2;
}

.label {
  @apply text-[11px] font-semibold text-text-tertiary uppercase;
  letter-spacing: 0.07em;
}

.card {
  @apply border border-border rounded-[10px] overflow-hidden;
}

/* The hairline between two rows is the second one's own: the slot's content is written by the pane,
   which is another component, so a scoped rule cannot reach it without `:deep`. */
.card :deep(.row + .row) {
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.note {
  @apply text-[11.5px] text-text-tertiary;
  line-height: 1.5;
}
</style>
