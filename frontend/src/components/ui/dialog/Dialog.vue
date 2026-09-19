<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, type DialogRootProps } from 'reka-ui'

// `class` lands on the panel, so sizing and padding stay with the caller. Escape closes
// unless the caller prevents it — the environments sheet does, for its own cascade.
defineOptions({ inheritAttrs: false })

// `center` is a dialog asking something, which stands in the middle of the window. `top` is a panel
// the user is working in while the window stays visible under it — the palette — and it hangs from
// the top edge where the design puts it. Both are the same primitive: what changes is where the
// panel is and which way it grows.
const props = defineProps<DialogRootProps & { title?: string; placement?: 'center' | 'top' }>()

// The flag travels with the event: reka-ui answers with whether the dialog is open now, and a caller
// holding `v-model:open` needs it — an event without it reads as "closed" whatever happened.
const emit = defineEmits<{ (e: 'update:open', open: boolean): void }>()
</script>

<template>
  <DialogRoot :open="props.open" @update:open="(open) => emit('update:open', open)">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent v-bind="$attrs" :class="['dialog', { 'dialog-top': props.placement === 'top' }]">
        <!-- A DialogTitle names the dialog for screen readers; a caller with its
             own header renders one itself inside the slot. -->
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
  animation: dialog-overlay-in 0.15s ease;
}

.dialog-overlay[data-state='closed'] {
  animation: dialog-overlay-out 0.12s ease-in forwards;
}

.dialog {
  @apply fixed left-1/2 top-1/2 max-w-[90vw] z-1500;
  transform: translate(-50%, -50%);
  background: var(--glass-overlay);
  backdrop-filter: var(--blur-overlay);
  border: 1px solid var(--glass-overlay-border);
  border-radius: 12px;
  box-shadow: var(--glass-overlay-shadow);
  animation: dialog-in 0.16s cubic-bezier(0.16, 1, 0.3, 1);
}

/* One dialog is not glass: the alert about unsaved changes is drawn as a system alert in the mockup —
   opaque, with the app's own shadow — and a frosted window over a card being edited would read as one
   more layer of the app rather than as the system asking. */
.dialog.system {
  background: var(--bg-panel);
  backdrop-filter: none;
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.dialog[data-state='closed'] {
  animation: dialog-out 0.12s ease-in forwards;
}

/* The palette hangs from the top edge and grows downward, so its transform is the centring alone —
   the same one the keyframes below carry, for the same reason: transform is already spoken for. */
.dialog-top {
  top: 92px;
  transform: translateX(-50%);
  animation: palette-in 0.16s cubic-bezier(0.16, 1, 0.3, 1);
}

.dialog-top[data-state='closed'] {
  animation: palette-out 0.12s ease-in forwards;
}

@keyframes palette-in {
  from {
    opacity: 0;
    transform: translateX(-50%) translateY(-8px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateX(-50%) translateY(0) scale(1);
  }
}

@keyframes palette-out {
  from {
    opacity: 1;
    transform: translateX(-50%) translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateX(-50%) translateY(-6px) scale(0.99);
  }
}

@keyframes dialog-overlay-in {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes dialog-overlay-out {
  from {
    opacity: 1;
  }
  to {
    opacity: 0;
  }
}

/* Centering is also a `transform`, so both keyframes carry it alongside the scale. */
@keyframes dialog-in {
  from {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
  }
}

@keyframes dialog-out {
  from {
    opacity: 1;
    transform: translate(-50%, -50%) scale(1);
  }
  to {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.97);
  }
}

/* No margin: whatever follows decides its own spacing. */
.dialog-title {
  @apply text-[14px] font-semibold;
}
</style>
