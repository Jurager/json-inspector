<script setup lang="ts">
// A row of colours to choose from: 20px circles, the chosen one ringed by its own tint. What the
// colours *are* is the caller's business — a word, the token it is drawn with, and the word a person
// reads in the tooltip — because the window that offers them is the one that knows its own palette.
//
// A shared control and not one per feature: the swatches of an environment and of a workspace are the
// same drawing in two windows, and the ring recipe is worth having in one place.
defineProps<{
  colors: readonly string[]
  /** The word Go stores, and the token it is drawn with. */
  tints: Record<string, string>
  /** The word a person reads — the `title` of a swatch. Empty leaves the tooltip off. */
  labels?: Record<string, string>
  value: string
  /** What the ring's inner stop is drawn on: the panel a field sits on, or a sheet's own leaf. */
  ground?: string
  disabled?: boolean
}>()

const emit = defineEmits<{ (e: 'pick', color: string): void }>()
</script>

<template>
  <div class="swatches">
    <button
      v-for="color in colors"
      :key="color"
      type="button"
      class="swatch"
      :class="{ chosen: value === color }"
      :disabled="disabled"
      :title="labels?.[color]"
      :style="{ background: tints[color], '--tint': tints[color], '--ground': ground }"
      @click="emit('pick', color)"
    ></button>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.swatches {
  @apply flex items-center gap-[7px] h-[34px];
}

.swatch {
  @apply w-5 h-5 flex-none rounded-full border-0 p-0 cursor-pointer;
}

.swatch:disabled {
  @apply opacity-50 cursor-default;
}

/* The ring is the tint itself, drawn twice around the dot: an opaque ground, then the colour — which
   is how the drawing marks the chosen one. The ground comes from the caller, because a translucent
   ring would let the circle standing on it show through the gap: a field's panel in one window, the
   leaf of the sheet in another. */
.swatch.chosen {
  box-shadow:
    0 0 0 2px var(--ground, var(--bg-panel)),
    0 0 0 3.5px var(--tint);
}
</style>
