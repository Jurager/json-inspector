<script setup lang="ts">
// A row of a list sheet: a name and a sentence on the left, and on the right either a tag saying
// what happens to it or a control of the caller's own. The browser page's capture rules put a field
// and a switch there; the confirmations put a tag.
const props = withDefaults(
  defineProps<{
    label: string
    note?: string
    tag?: string
    // A tag is either something the sheet is going to do or something it is going to leave alone,
    // and the pair of fills is what says which.
    tone?: 'keep' | 'drop'
  }>(),
  { note: '', tag: '', tone: 'drop' }
)
</script>

<template>
  <div class="sheet-row">
    <span class="row-text">
      <span class="row-label">{{ props.label }}</span>
      <span v-if="props.note" class="row-note">{{ props.note }}</span>
    </span>
    <span v-if="props.tag" class="row-tag" :class="props.tone">{{ props.tag }}</span>
    <slot v-else />
  </div>
</template>

<style scoped>
@reference "../../../style.css";

/* A row of a sheet is a line of a list and not a table row: 44px, its own radius, and a fill that
   appears only under the pointer. */
.sheet-row {
  @apply flex items-center gap-3 min-h-11 px-2.5 rounded-[9px];
}

.sheet-row:hover {
  @apply bg-bg-hover;
}

.row-text {
  @apply flex-1 min-w-0 flex flex-col gap-0.5;
}

.row-label {
  @apply text-[13.5px];
}

.row-note {
  @apply text-[12px] text-text-tertiary;
}

.row-tag {
  @apply flex-none text-[12.5px] font-semibold px-[9px] py-1 rounded-md
         bg-bg-hover text-text-tertiary;
}

.row-tag.keep {
  @apply bg-green-soft;
  color: var(--green-text);
}
</style>
