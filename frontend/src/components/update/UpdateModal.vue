<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import { useUpdates } from '../../composables/useUpdates'

// State and actions come from the composable, so the rail's "Проверить
// обновления" and the status bar's link open this same dialog.
const { update, updating, modalOpen, close, apply } = useUpdates()
</script>

<template>
  <Dialog
    v-if="update"
    :open="modalOpen"
    title="Доступна новая версия"
    class="w-90 p-4.5"
    @update:open="close"
  >
    <div class="body">
      Версия <b>{{ update.latest }}</b> (у вас {{ update.current }}).<br />
      Обновить сейчас? Приложение перезапустится.
    </div>
    <div class="actions">
      <Button @click="close">Позже</Button>
      <Button variant="primary" :disabled="updating" @click="apply">
        {{ updating ? 'Обновление…' : 'Обновить' }}
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
