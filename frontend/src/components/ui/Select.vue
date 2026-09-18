<script setup lang="ts">
// A choice out of a list, as the drawing makes it: the platform's own dropdown in the app's clothes.
// Native rather than a reka menu, because a settings row's list is long, scrolls on its own on a
// small window, and is the one control on the screen a keyboard already knows how to reach.
const props = withDefaults(
  defineProps<{
    modelValue: string
    options: { value: string; label: string }[]
    disabled?: boolean
  }>(),
  { disabled: false }
)

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

function pick(e: Event) {
  emit('update:modelValue', (e.target as HTMLSelectElement).value)
}
</script>

<template>
  <span class="select" :class="{ disabled }">
    <select :value="props.modelValue" :disabled="disabled" @change="pick">
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
    <svg
      class="chevron"
      width="11"
      height="11"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2.6"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  </span>
</template>

<style scoped>
@reference "../../style.css";

.select {
  @apply relative inline-flex flex-none items-center;
}

select {
  @apply h-7 rounded-[7px] text-[13px] text-text cursor-pointer appearance-none;
  /* The app's own field: a panel fill under a heavier hairline than the row around it. */
  font: inherit;
  font-size: 13px;
  min-width: 140px;
  padding: 0 30px 0 10px;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

select:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.select.disabled select,
select:disabled {
  @apply opacity-50 cursor-default;
}

/* Drawn over the field rather than left to the platform: the arrow a select carries is a different
   shape in every engine, and a row of them would be a row of four different drawings. */
.chevron {
  @apply absolute right-[9px] text-text-tertiary pointer-events-none;
}
</style>
