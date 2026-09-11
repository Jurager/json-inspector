<script setup lang="ts">
// A square button whose only content is a glyph — reka-ui has none either.
// Metrics from the handoff: the toolbar tile is 24px with a 6px radius, the
// small glyph buttons (delete, reveal-secret) are 20px with a 4px radius, and
// the rail's own buttons are 36px with 9px. The four variants are its four
// species of glyph button, and its colours come with them: outline is the
// bordered tile ("назад" in the response bar, rail tiles), bare is the same
// glyph at --text-secondary (the rail's menu button, the environment sheet's
// +/–), subtle is the dimmer one used for closes, danger is subtle turning
// --red on hover (row deletes).
//
// A glyph has no label, so `hint` becomes a styled tooltip. On a disabled
// button it falls back to the native title instead: a disabled element fires no
// pointer events, so there'd be nothing for the tooltip to catch, and the
// explanation of *why* it is disabled is exactly what matters there.
//
// `hint` is not usable on a button that is itself an `as-child` trigger (the
// rail's menu button): the tooltip's root is a provider, so the trigger's props
// would land on the provider instead of the button and the menu would stop
// opening.
import Tooltip from '../tooltip/Tooltip.vue'

withDefaults(
  defineProps<{
    variant?: 'outline' | 'bare' | 'subtle' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    hint?: string
    disabled?: boolean
  }>(),
  { variant: 'subtle', size: 'md' }
)

// Not inherited: with a hint the root is the tooltip, and the click handler the
// caller passes would land on the tooltip's panel instead of on the button.
// Both branches below hand `$attrs` to a button explicitly.
defineOptions({ inheritAttrs: false })
</script>

<template>
  <Tooltip v-if="hint && !disabled" side="top">
    <template #trigger>
      <button
        v-bind="$attrs"
        type="button"
        :class="['icon-btn', `icon-btn--${variant}`, `icon-btn--${size}`]"
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
  width: 36px;
  height: 36px;
  border-radius: 9px;
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
