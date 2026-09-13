<script setup lang="ts">
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import Icon from '../ui/Icon.vue'
import { useCollectionsStore } from '../../stores/collections'

// The alert that stands between a click and a card with unsaved edits. It is drawn once, at the top
// of the window, because what asks is not always the tree: the rail and the window's close ask too.
const store = useCollectionsStore()
</script>

<template>
  <Dialog
    :open="store.pendingLeave !== null"
    class="w-[420px] p-5"
    @escape-key-down.prevent
    @update:open="store.answerUnsaved('cancel')"
  >
    <div class="head">
      <span class="icon"><Icon name="bookmark" :size="16" /></span>
      <span class="title">Сохранить изменения в «{{ store.pendingLeave?.name }}»?</span>
    </div>
    <p class="hint">
      Правка не сохранена в коллекции: «Отправить» её не записывает. Если не сохранить, изменения
      потеряются.
    </p>
    <div class="actions">
      <Button class="discard" @click="store.answerUnsaved('discard')">Не сохранять</Button>
      <span class="spacer"></span>
      <Button @click="store.answerUnsaved('cancel')">Отмена</Button>
      <Button variant="primary" @click="store.answerUnsaved('save')">Сохранить</Button>
    </div>
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

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
  @apply mt-2.5 mb-4 text-[12.5px] text-text-secondary;
}

.actions {
  @apply flex items-center gap-2;
}

/* «Не сохранять» stands apart on the left: the one button that throws something away is not the one
   a hand reaches for by accident. */
.discard {
  @apply text-red;
}

.spacer {
  @apply flex-1;
}
</style>
