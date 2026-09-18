<script setup lang="ts">
// One line of a settings section: what the row is, what it is about, and the one control that decides
// it. The hairline above it belongs to the row and is drawn by the section, so a section of one row
// draws none.
withDefaults(defineProps<{ title: string; note?: string }>(), { note: '' })
</script>

<template>
  <div class="row">
    <span class="text">
      <span class="title">{{ title }}</span>
      <span v-if="note" class="note">{{ note }}</span>
    </span>
    <span class="control"><slot /></span>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.row {
  @apply flex items-center gap-3.5;
  min-height: 44px;
  padding: 9px 12px;
}

.text {
  @apply flex-1 min-w-0 flex flex-col;
  gap: 2px;
}

.title {
  @apply text-[13px];
}

.note {
  @apply text-[12px] text-text-tertiary;
  line-height: 1.45;
}

/* The right-hand side takes whatever the row puts there — one control, or a value and the button that
   opens what it is about — and never grows: a flex-1 control would push the words out of the row. */
.control {
  @apply flex-none flex items-center gap-2;
}
</style>
