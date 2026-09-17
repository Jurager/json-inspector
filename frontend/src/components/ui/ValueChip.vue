<script setup lang="ts">
// A value the screen states rather than offers: a panel width, the path of the data folder. It is
// drawn with a field's fill and hairline so that a row of controls reads as one line — a plain
// sentence on the right of a row would look like part of the title it stands beside.
//
// Read-only by design, and the whole of it is the tooltip: what it shows is often a path too long for
// the column, and an ellipsis with nothing behind it is a value nobody can read.
defineProps<{
  value: string
  mono?: boolean
}>()
</script>

<template>
  <span class="chip" :class="{ mono }" :title="value">{{ value }}</span>
</template>

<style scoped>
@reference "../../style.css";

.chip {
  @apply inline-flex flex-none items-center h-7 px-2.5 rounded-[7px] text-[12.5px]
         text-text-secondary;
  max-width: 240px;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: var(--mono);
  font-size: 12px;
}
</style>
