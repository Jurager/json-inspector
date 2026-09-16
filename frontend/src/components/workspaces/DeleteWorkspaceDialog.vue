<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import { useMessages } from '../../i18n'

defineProps<{ name: string; open: boolean }>()

const emit = defineEmits<{ (e: 'cancel'): void; (e: 'confirm'): void }>()

const { t } = useMessages()
</script>

<template>
  <Dialog
    :open="open"
    :title="t('workspaces.deleteTitle')"
    class="w-90 p-4.5"
    @escape-key-down.prevent
    @update:open="emit('cancel')"
  >
    <div class="body">
      {{ t('workspaces.deleteBodyHead') }} <b>{{ name }}</b>{{ t('workspaces.deleteBodyTail') }}
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
  @apply mt-3 mb-4 text-[13px] text-text-secondary;
}

.actions {
  @apply flex justify-end gap-2;
}
</style>
