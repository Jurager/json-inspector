<script setup lang="ts">
import Icon from './Icon.vue'

const model = defineModel<string>({ required: true })

defineProps<{ placeholder: string }>()
</script>

<template>
  <div class="dock">
    <div class="fade"></div>
    <label class="strip">
      <Icon name="search" :size="12" :stroke-width="1.8" />
      <input
        v-model="model"
        class="entry"
        :placeholder="placeholder"
        spellcheck="false"
        aria-label="placeholder"
      />
    </label>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The strip is the last 40px of the panel and is opaque, so it is a footer rather than a floating
   field: the list scrolls behind the 18px fade above it and stops at the hairline. */
.dock {
  /* Only the strip takes the pointer: the fade sits over the last row of the list, and a click there
     belongs to the row. */
  @apply absolute left-0 right-0 bottom-0 flex flex-col pointer-events-none;
}

.fade {
  height: 18px;
  background: linear-gradient(to bottom, transparent, var(--bg-panel));
  pointer-events: none;
}

.strip {
  @apply flex-none h-10 flex items-center gap-1.5 px-3.5 bg-bg-panel border-t border-border cursor-text pointer-events-auto;
  color: var(--text-tertiary);
}

.entry {
  @apply flex-1 min-w-0 text-[12px] text-text bg-transparent border-none outline-none p-0;
  font-family: inherit;
}

.entry::placeholder {
  @apply text-text-tertiary;
}
</style>
