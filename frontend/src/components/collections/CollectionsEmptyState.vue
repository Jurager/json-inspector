<script setup lang="ts">
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { useCollectionsStore } from '../../stores/collections'
import { describeFailure, useMessages } from '../../i18n'
import { useToast } from '../../composables/useToast'

// Until the first collection exists there is nothing to list, so the window says what a collection
// is and offers the one gesture that starts one — the panel with a lone «+» would say less.
const store = useCollectionsStore()

const { t } = useMessages()
const toast = useToast()

function create() {
  void store.createCollection(t('collections.newCollection'))
}

async function importCollection() {
  try {
    const name = await store.importFile()
    if (name) toast.show(t('collections.imported', { name }))
  } catch (error) {
    toast.show(t('collections.importFailed', { error: describeFailure(error) }), 'error')
  }
}
</script>

<template>
  <div class="onboarding">
    <span class="title">{{ t('collections.title') }}</span>
    <span class="hint">
      {{ t('collections.emptyHint') }}
    </span>
    <div class="actions">
      <Button variant="primary" @click="create">
        <Icon name="plus" :size="16" /> {{ t('collections.newCollection') }}
      </Button>
      <Button @click="importCollection">
        <Icon name="download" :size="14" /> {{ t('collections.importCollection') }}
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
  @apply mt-2 flex items-center justify-center gap-2;
}
</style>
