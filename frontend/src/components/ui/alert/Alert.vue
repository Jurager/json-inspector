<script setup lang="ts">
import Dialog from '../dialog/Dialog.vue'
import Icon from '../Icon.vue'

// The alert that stands between a click and unsaved work. One frame for the two places that ask —
// a collection card and the command line — because the question is the same; the answers are the
// caller's, and it is the only side that knows what discarding throws away. A card saves back into
// the node it came from; the line has no collection of its own, so there the way to keep the work
// is a button beside the address, and the hint says so.
defineProps<{
  open: boolean
  icon: string
  title: string
  hint: string
}>()

// Closing — Escape, or a click outside — is always the answer that keeps what is there, and it is
// the caller that decides what that means.
const emit = defineEmits<{ (e: 'cancel'): void }>()
</script>

<template>
  <Dialog
    :open="open"
    class="w-[420px] p-5 system"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <div class="head">
      <span class="icon"><Icon :name="icon" :size="16" /></span>
      <span class="title">{{ title }}</span>
    </div>
    <p class="hint">{{ hint }}</p>
    <div class="actions">
      <slot />
    </div>
  </Dialog>
</template>

<style scoped>
@reference "../../../style.css";

.head {
  @apply flex items-center gap-2.5;
}

.icon {
  @apply flex-none inline-flex items-center justify-center w-7 h-7 rounded-lg text-accent bg-accent-soft;
}

.title {
  @apply text-[14px] font-semibold;
}

.hint {
  @apply mt-2.5 mb-4 text-[13px] text-text-secondary;
}

.actions {
  @apply flex items-center gap-2;
}
</style>
