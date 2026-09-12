<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle, type DialogRootProps } from 'reka-ui'

// `class` lands on the panel, so sizing and padding stay with the caller. Escape closes
// unless the caller prevents it — the environments sheet does, for its own cascade.
defineOptions({ inheritAttrs: false })

const props = defineProps<DialogRootProps & { title?: string }>()

const emit = defineEmits<{ (e: 'update:open'): void }>()
</script>

<template>
  <DialogRoot :open="props.open" @update:open="emit('update:open')">
    <DialogPortal>
      <DialogOverlay class="dialog-overlay" />
      <DialogContent v-bind="$attrs" class="dialog">
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
}

.dialog {
  @apply fixed left-1/2 top-1/2 max-w-[90vw] z-1500;
  transform: translate(-50%, -50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: var(--shadow);
}

/* No margin: whatever follows decides its own spacing. */
.dialog-title {
  @apply text-[15px] font-semibold;
}
</style>
