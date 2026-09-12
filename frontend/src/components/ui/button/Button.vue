<script setup lang="ts">
// Sizes are the handoff's: md is «Готово», lg is the command line's send button, sm is the
// small button for tight rows. ghost and quiet size by padding, not by height.
withDefaults(
  defineProps<{
    variant?: 'outline' | 'primary' | 'ghost' | 'quiet'
    size?: 'sm' | 'md' | 'lg'
  }>(),
  { variant: 'outline', size: 'md' }
)
</script>

<template>
  <button type="button" :class="['btn', `btn--${variant}`, `btn--${size}`]">
    <slot />
  </button>
</template>

<style scoped>
@reference "../../../style.css";

.btn {
  /* flex-none: in a toolbar the spacer takes the slack, not the buttons. */
  @apply inline-flex flex-none items-center justify-center cursor-pointer whitespace-nowrap bg-bg-panel text-text;
  border: 1px solid var(--border-strong);
  box-shadow: var(--shadow-btn);
  font: inherit;
  transition: background 0.12s ease, border-color 0.12s ease, filter 0.12s ease;
  --wails-draggable: no-drag;
}

.btn:hover:not(:disabled) {
  @apply bg-bg-hover;
}

.btn:active:not(:disabled) {
  @apply bg-bg-active;
}

.btn:disabled {
  @apply opacity-50 cursor-default shadow-none;
}

.btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
}

/* Gap is the handoff's own per-size value. */
.btn--md {
  gap: 6px;
  height: 26px;
  padding: 0 12px;
  border-radius: 7px;
  font-size: 12px;
}

.btn--sm {
  gap: 5px;
  height: 24px;
  padding: 0 9px;
  border-radius: 6px;
  font-size: 11.5px;
}

.btn--lg {
  gap: 7px;
  height: 32px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12.5px;
  font-weight: 500;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.18);
}

.btn--primary {
  @apply bg-accent border-accent text-accent-text;
}

.btn--primary:hover:not(:disabled) {
  @apply bg-accent border-accent brightness-110;
}

.btn--primary:active:not(:disabled) {
  @apply brightness-95;
}

.btn.btn--ghost,
.btn.btn--quiet {
  @apply border-transparent bg-transparent shadow-none;
  height: auto;
  font-size: 12px;
}

.btn--ghost {
  @apply text-accent;
  padding: 3px 4px;
  border-radius: 5px;
}

.btn--ghost.btn--md,
.btn--quiet.btn--md {
  padding: 4px 8px;
  border-radius: 6px;
}

.btn--quiet {
  @apply text-text-secondary;
  padding: 4px 8px;
  border-radius: 6px;
}

.btn--ghost:hover:not(:disabled) {
  @apply bg-accent-soft;
}

.btn--ghost:active:not(:disabled) {
  @apply bg-accent-soft brightness-95;
}

.btn--quiet:hover:not(:disabled) {
  @apply bg-bg-hover text-text;
}
</style>
