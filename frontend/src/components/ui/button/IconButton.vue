<script setup lang="ts">
// `hint` is unusable on a button that is itself an `as-child` trigger (the rail's menu): the
// tooltip's root is a provider and the trigger's props would land on it.
import { ref } from 'vue'
import Tooltip from '../tooltip/Tooltip.vue'
import { useHoverArrival } from '../../../composables/useHoverArrival'

withDefaults(
  defineProps<{
    variant?: 'outline' | 'bare' | 'subtle' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    hint?: string
    disabled?: boolean
  }>(),
  { variant: 'subtle', size: 'md' }
)

// With a hint the root is the tooltip, so a caller's click handler would land on
// the tooltip's panel; both branches below hand `$attrs` to a button explicitly.
defineOptions({ inheritAttrs: false })

const hintEl = ref<HTMLElement | null>(null)
const hintArmed = useHoverArrival(hintEl)
</script>

<template>
  <Tooltip v-if="hint && !disabled" side="top" :disabled="!hintArmed">
    <template #trigger>
      <button
        ref="hintEl"
        v-bind="$attrs"
        type="button"
        :class="['icon-btn', `icon-btn--${variant}`, `icon-btn--${size}`]"
        @pointerleave="hintArmed = true"
      >
        <slot />
      </button>
    </template>
    {{ hint }}
  </Tooltip>

  <button
    v-else
    v-bind="$attrs"
    type="button"
    :disabled="disabled"
    :title="hint"
    :class="['icon-btn', `icon-btn--${variant}`, `icon-btn--${size}`]"
  >
    <slot />
  </button>
</template>

<style scoped>
@reference "../../../style.css";

.icon-btn {
  @apply inline-flex flex-none items-center justify-center border-none bg-transparent cursor-pointer;
  transition: color 0.12s ease, background 0.12s ease, border-color 0.12s ease;
  --wails-draggable: no-drag;
}

.icon-btn:disabled {
  @apply opacity-50 cursor-default;
}

.icon-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

.icon-btn--sm {
  width: 20px;
  height: 20px;
  border-radius: 4px;
}

.icon-btn--md {
  width: 24px;
  height: 24px;
  border-radius: 6px;
}

.icon-btn--lg {
  width: 32px;
  height: 32px;
  border-radius: 8px;
}

.icon-btn--outline {
  @apply text-text-secondary;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
  box-shadow: var(--shadow-btn);
}

.icon-btn--outline:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}

.icon-btn--bare {
  @apply text-text-secondary;
}

.icon-btn--bare:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}

.icon-btn--subtle,
.icon-btn--danger {
  @apply text-text-tertiary;
}

.icon-btn--subtle:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}

.icon-btn--danger:hover:not(:disabled) {
  @apply bg-bg-hover text-red;
}
</style>
