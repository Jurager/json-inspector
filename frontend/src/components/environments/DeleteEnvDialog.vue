<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import { useMessages } from '../../i18n'

const { t } = useMessages()

defineProps<{ name: string; open: boolean }>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'confirm'): void }>()
</script>

<template>
  <Dialog
    :open="open"
    :title="t('environments.deleteTitle')"
    class="w-90 p-4.5"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <div class="body">
      {{ t('environments.deleteBodyHead') }} <b>{{ name }}</b>{{ t('environments.deleteBodyTail') }}
    </div>
    <div class="actions">
      <Button @click="emit('cancel')">{{ t('common.cancel') }}</Button>
      <Button variant="primary" @click="emit('confirm')">{{ t('common.delete') }}</Button>
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
