<script setup lang="ts">
import Icon from './Icon.vue'

const model = defineModel<string>({ required: true })

defineProps<{ placeholder: string }>()
</script>

<template>
  <label class="strip">
    <Icon name="search" :size="16" :stroke-width="1.8" />
    <input
      v-model="model"
      class="entry"
      :placeholder="placeholder"
      spellcheck="false"
      aria-label="placeholder"
    />
  </label>
</template>

<style scoped>
@reference "../../style.css";

/* The last 54px of the panel, and a footer rather than a floating field: the list ends where it
   begins, so no fade and no overlay — the row above the strip is a row like any other. */
.strip {
  @apply flex-none h-[55px] flex items-center gap-[9px] px-4 bg-bg-panel border-t border-border cursor-text;
  color: var(--text-tertiary);
}

.entry {
  @apply flex-1 min-w-0 text-[13px] text-text bg-transparent border-none outline-none p-0;
  font-family: inherit;
}

.entry::placeholder {
  @apply text-text-tertiary;
}
</style>
