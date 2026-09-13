<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import { plural } from '../../lib/format'

defineProps<{ name: string; count: number; open: boolean }>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'confirm'): void }>()
</script>

<template>
  <Dialog
    :open="open"
    title="Удалить вместе с содержимым?"
    class="w-110 p-4.5"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <div class="body">
      В коллекции {{ count }} {{ plural(count, ['запрос', 'запроса', 'запросов']) }} — они удалятся вместе с ней.
    </div>
    <div class="actions">
      <Button @click="emit('cancel')">Отмена</Button>
      <Button variant="primary" @click="emit('confirm')">Удалить</Button>
    </div>
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

.body {
  @apply mt-3 mb-4 text-[13px] text-text-secondary;
}

.actions {
  @apply flex justify-end gap-2;
}
</style>
