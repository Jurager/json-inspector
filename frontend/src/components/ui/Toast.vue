<script setup lang="ts">
import { useToast } from '../../composables/useToast'

// Display only: whoever needs to say something calls useToast().show(...).
const { message, kind } = useToast()
</script>

<template>
  <transition name="fade">
    <div v-if="message" class="toast" :class="kind">{{ message }}</div>
  </transition>
</template>

<style scoped>
@reference "../../style.css";

.toast {
  @apply fixed bottom-4 left-1/2 px-4 py-2 rounded-lg text-xs z-2000 max-w-[80%];
  transform: translateX(-50%);
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: var(--shadow);
}

.toast.error {
  @apply text-red;
}

.toast.info {
  @apply text-text;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
