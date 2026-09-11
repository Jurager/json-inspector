<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, type DialogRootProps } from 'reka-ui'

// Focus trap, scroll lock, Escape, click-outside and the ARIA wiring are
// reka-ui's; the backdrop and the panel are our classes, so a dialog built on
// this looks like the hand-rolled overlays it replaces.
//
// `class` lands on the panel (see the attribute forwarding below), so sizing
// stays with the caller: w-90 for a confirmation, a fixed 620px for the import
// review, and so on. Padding is the caller's too — the import dialog puts its
// own header and footer flush against the edges.
//
// Escape closes the dialog unless the caller prevents it. Inside the
// environments sheet one Escape backs out a single level, and two listeners —
// reka-ui closing the dialog and the sheet's own cascade closing the sheet —
// would take two levels at once. Those call sites pass @escape-key-down.prevent
// and let the sheet's cascade decide.
defineOptions({ inheritAttrs: false })

const props = defineProps<DialogRootProps & { title?: string }>()

const emit = defineEmits<{ (e: 'update:open'): void }>()
</script>

<template>
  <DialogRoot :open="props.open" @update:open="emit('update:open')">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent v-bind="$attrs" class="dialog">
        <!-- A DialogTitle is what names the dialog for screen readers; a caller
             with its own header renders one itself inside the slot. -->
        <DialogTitle v-if="props.title" class="dialog-title">{{ props.title }}</DialogTitle>
        <slot />
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
@reference "../../../style.css";

.dialog-overlay {
  @apply fixed inset-0 z-1500;
  background: rgba(0, 0, 0, 0.4);
}

.dialog {
  @apply fixed left-1/2 top-1/2 max-w-[90vw] z-1500;
  transform: translate(-50%, -50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: var(--shadow);
}

/* No margin: whatever follows decides its own spacing, which keeps the title
   usable for callers with a full-width header of their own. */
.dialog-title {
  @apply text-[15px] font-semibold;
}
</style>
