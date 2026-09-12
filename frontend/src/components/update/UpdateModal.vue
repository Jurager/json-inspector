<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import { useUpdates } from '../../composables/useUpdates'

// Shared composable state, so the rail's "Проверить обновления" opens this same dialog.
const { availableUpdate, isInstalling, isModalOpen, closeModal, installUpdate } = useUpdates()
</script>

<template>
  <Dialog
    v-if="availableUpdate"
    :open="isModalOpen"
    title="Доступна новая версия"
    class="w-90 p-4.5"
    @update:open="closeModal"
  >
    <div class="body">
      Версия <b>{{ availableUpdate.latest }}</b> (у вас {{ availableUpdate.current }}).<br />
      Обновить сейчас? Приложение перезапустится.
    </div>
    <div class="actions">
      <Button @click="closeModal">Позже</Button>
      <Button variant="primary" :disabled="isInstalling" @click="installUpdate">
        {{ isInstalling ? 'Обновление…' : 'Обновить' }}
      </Button>
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
