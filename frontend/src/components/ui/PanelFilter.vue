<script setup lang="ts">
import Icon from './Icon.vue'

// The filter of a list panel, docked at its foot. The history and the collections draw the same one —
// the design gives both a grey pill with a magnifier on 26px of gradient — so it is one component and
// two callers rather than two copies that drift apart.
const model = defineModel<string>({ required: true })

defineProps<{ placeholder: string }>()
</script>

<template>
  <div class="dock">
    <div class="fade"></div>
    <div class="field">
      <div class="pill">
        <Icon name="search" :size="12" />
        <input
          v-model="model"
          class="entry"
          :placeholder="placeholder"
          spellcheck="false"
          aria-label="placeholder"
        />
      </div>
    </div>
    <div class="backdrop"></div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.dock {
  @apply absolute left-0 right-0 bottom-0 flex flex-col pointer-events-none;
}

/* The rows scroll away under this: 26px of the panel's own colour, ending in it. */
.fade {
  height: 26px;
  background: linear-gradient(
    to bottom,
    color-mix(in srgb, var(--bg-panel) 0%, transparent) 0%,
    color-mix(in srgb, var(--bg-panel) 3%, transparent) 10%,
    color-mix(in srgb, var(--bg-panel) 10%, transparent) 20%,
    color-mix(in srgb, var(--bg-panel) 22%, transparent) 30%,
    color-mix(in srgb, var(--bg-panel) 35%, transparent) 40%,
    color-mix(in srgb, var(--bg-panel) 50%, transparent) 50%,
    color-mix(in srgb, var(--bg-panel) 65%, transparent) 60%,
    color-mix(in srgb, var(--bg-panel) 78%, transparent) 70%,
    color-mix(in srgb, var(--bg-panel) 90%, transparent) 80%,
    color-mix(in srgb, var(--bg-panel) 97%, transparent) 90%,
    var(--bg-panel) 100%
  );
}

.field {
  @apply pointer-events-auto px-2 pb-2 bg-bg-panel;
}

/* A pill rather than a field: the design's filter has no border, because it sits on the panel it
   filters and a second rectangle in a list of rows reads as one more row. */
.pill {
  @apply flex items-center gap-1.5 h-8 px-[9px] rounded-[7px] text-text-tertiary;
  background: color-mix(in srgb, var(--text) 4.5%, transparent);
  backdrop-filter: blur(12px);
}

/* The design's 12px, and the panel's own font: the sizing cannot come from `font: inherit`, which
   would reset it to the 13px the window is set in. */
.entry {
  @apply flex-1 min-w-0 text-[12px] text-text bg-transparent border-none outline-none p-0;
  font-family: inherit;
}

.entry::placeholder {
  @apply text-text-tertiary;
}

.backdrop {
  @apply h-2 bg-bg-panel;
}
</style>
