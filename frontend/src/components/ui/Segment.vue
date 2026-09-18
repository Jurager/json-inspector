<script setup lang="ts">
// A row of exclusive choices: what a new environment starts from, whether this one is editable, what
// a workspace is meant to be, and every choice a settings row offers. One control for all of them,
// because they are the same drawing with different words in it.
//
// An option may be drawn and not offered: `soon` is a choice the design shows so that the answer it
// will have is visible, and `soonHint` is what says so on hover. It never emits, and it is disabled
// on its own, so a row can hold one live choice beside one that is still coming.
withDefaults(
  defineProps<{
    options: { value: string; label: string; soon?: boolean }[]
    value: string
    /** `sm` is the settings rows' height: the drawing gives them a hair less than a form's. */
    size?: 'md' | 'sm'
    /** Whether the choices share the width equally. A settings row wants them as wide as their
     * words, a form wants them to fill what it gives them. */
    grow?: boolean
    soonHint?: string
    disabled?: boolean
  }>(),
  { size: 'md', grow: true, soonHint: '', disabled: false }
)

const emit = defineEmits<{ (e: 'pick', value: string): void }>()
</script>

<template>
  <div class="segment" :class="[`segment--${size}`, { grow, disabled }]">
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

/* A block rather than an inline box, as it was: a form's segment is told its width by the form and
   the choices share it, and a settings row keeps it content-sized because that row is a flex line. */
.segment {
  @apply flex gap-0.5 p-0.5 rounded-lg bg-bg-hover;
}

.seg {
  @apply min-w-0 text-center border-none rounded-md cursor-pointer
         bg-transparent text-text-secondary;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  padding: 0 11px;
  /* A segment is one line by construction: a name that wrapped would make the row two rows tall and
     shove what stands under it out of place. A long name ellipsizes instead. */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
}

/* The choices share the width equally only when the caller asks for it: in a form they fill what it
   gives them, and in a settings row they are as wide as their own words. */
.segment.grow .seg {
  @apply flex-1;
}

/* The chosen segment is the panel's own colour lifted off the groove it sits in. */
.seg.active {
  @apply bg-bg-panel text-text;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.12);
}

.segment--md .seg {
  height: 28px;
}

.segment--sm .seg {
  height: 26px;
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
