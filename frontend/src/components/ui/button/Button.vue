<script setup lang="ts">
// Sizes are the handoff's: md is «Готово», lg is the command line's send button, sm is the
// small button for tight rows. ghost and quiet size by padding, not by height.
//
// xl is the command line's send button as the latest handoff draws it: the strip is built around a
// 40px field, and the button that stands at its end is that tall too. It is a size of its own rather
// than a bigger lg because the rest of the window — the dialogs, the settings, the collection
// overview — still measures by the sizes above.
withDefaults(
  defineProps<{
    variant?: 'outline' | 'primary' | 'ghost' | 'quiet' | 'danger'
    size?: 'sm' | 'md' | 'lg' | 'xl' | 'xl-quiet' | 'panel' | 'page' | 'bar' | 'field'
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
  font-size: 12px;
}

.btn--lg {
  gap: 7px;
  height: 32px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.18);
}

.btn--xl {
  gap: 8px;
  height: 40px;
  padding: 0 18px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.18);
}

/* The button that stands beside the accent one at the same height, which is how the handoff draws a
   page's pair: the action and the quieter one next to it. The same 40px and radius as xl, read at
   13 rather than 14 and with a little less room around the label. */
.btn.btn--xl-quiet {
  gap: 9px;
  height: 40px;
  padding: 0 16px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
}

.btn--primary {
  @apply bg-accent border-accent text-accent-text;
}

/* The page head's button: 34px, between lg and xl, standing beside a page's title rather than in a bar
   of controls. */
.btn.btn--page {
  gap: 7px;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
}

/* And its fill is the handoff's own for a head: `--raise` under a `--line` hairline with no shadow —
   the button stands on the page rather than lying on it. The accent variant is excepted because it is
   the page's one action and wears the accent: written as `:not()` rather than settled by source
   order, because a rule that depends on where it is written moves the day the file is reordered. */
.btn.btn--page:not(.btn--primary) {
  background: var(--bg-inset);
  border-color: var(--border);
  box-shadow: none;
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

/* The chip popovers' button: 30px, standing in a row beside fields and 26px icon buttons. It is
   written against `.btn` itself, and last, because the ghost and quiet variants size themselves too
   and a bare size class would lose to them. */
.btn.btn--panel {
  gap: 7px;
  height: 30px;
  padding: 0 10px;
  border-radius: 7px;
  font-size: 13px;
  font-weight: 500;
}

/* The response header's button, and the handoff draws it a shade apart from the one above: the same
   13px, but 28px tall and at the tighter radius, because it lies on the bar rather than in a panel.
   Compare, Copy, Inspector and Search are all this one control, wherever the window puts them, so
   the fill is the handoff's too: `--raise` under a `--line` hairline and no shadow, which is what
   makes the row read as controls standing on the bar rather than as cards in it. */
.btn.btn--bar {
  gap: 7px;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  background: var(--bg-inset);
  border-color: var(--border);
  box-shadow: none;
}

/* The settings rows' button: the height and the radius of the field it stands beside. Geometry only —
   the fill is the variant's, which is why nothing here paints: a size that painted would beat
   `btn--primary` (two classes against one) and leave a white label on a white button, which is how
   the one danger button in this window was drawn invisible the first time round. No shadow either:
   the drawing has the button stand in the row rather than lift off it. */
.btn.btn--field {
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border-radius: 7px;
  font-size: 13px;
  font-weight: 500;
  box-shadow: none;
}

/* A row that throws something away: the colour of the danger strip under an environment's variables
   or a workspace's delete, so the same act looks the same wherever the window puts it. */
.btn--danger {
  color: var(--red-text);
  border-color: var(--red-soft);
  background: transparent;
}

.btn--danger:hover:not(:disabled) {
  background: var(--red-soft);
}
</style>
