<script setup lang="ts">
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { useCollectionsStore } from '../../stores/collections'

// Until the first collection exists there is nothing to list, so the window says what a collection
// is and offers the one gesture that starts one — the panel with a lone «+» would say less.
const store = useCollectionsStore()

function create() {
  void store.createCollection('Новая коллекция')
}
</script>

<template>
  <div class="onboarding">
    <span class="title">Коллекции</span>
    <span class="hint">
      Сохраняйте запросы в коллекции и папки — и запускайте их все одной кнопкой, по очереди.
    </span>
    <div class="actions">
      <Button variant="primary" @click="create">
        <Icon name="plus" :size="14" /> Новая коллекция
      </Button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.onboarding {
  @apply flex-1 min-h-0 flex flex-col items-center justify-center gap-2 px-8;
}

.title {
  @apply text-[15px] font-semibold;
}

.hint {
  @apply max-w-[380px] text-center text-[12.5px] text-text-secondary;
}

.actions {
  @apply mt-2;
}
</style>
