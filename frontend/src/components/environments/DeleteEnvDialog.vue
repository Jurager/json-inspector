<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'

defineProps<{ name: string; open: boolean }>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'confirm'): void }>()
</script>

<template>
  <Dialog
    :open="open"
    title="Удалить окружение?"
    class="w-90 p-4.5"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <div class="body">
      На переменные окружения <b>{{ name }}</b> ссылается сохранённый запрос. После удаления его токены станут неизвестными.
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
  @apply mt-3 text-[13px] text-text-secondary mb-4;
}

.actions {
  @apply flex justify-end gap-2;
}
</style>
