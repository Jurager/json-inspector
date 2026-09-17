<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button } from '../ui/button'
import { useCollectionsStore } from '../../stores/collections'
import { useWorkspacesStore } from '../../stores/workspaces'
import { describeFailure, useMessages } from '../../i18n'
import { useToast } from '../../composables/useToast'

// Until the first collection exists there is nothing to list, so the window says what a collection
// is and offers the two gestures that start one — the panel with a lone «+» would say less.
const store = useCollectionsStore()
const workspaces = useWorkspacesStore()

const { t } = useMessages()
const toast = useToast()

// The page names the space it is about: a window can hold several of them, and «no collections» is a
// sentence that needs its subject when the one on screen is not the only one there is.
const space = computed(() => workspaces.active?.name ?? '')

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
  <div class="start-page">
    <div class="start-card">
      <span class="start-icon"><Icon name="folder" :size="26" :stroke-width="1.6" /></span>

      <div class="start-text">
        <span class="start-title">{{ t('collections.emptyTitle', { name: space }) }}</span>
        <span class="start-body">{{ t('collections.emptyBody') }}</span>
      </div>

      <div class="start-actions">
        <Button variant="primary" size="xl" @click="create">
          <Icon name="plus" :size="15" :stroke-width="2.2" />
          {{ t('collections.newCollection') }}
        </Button>
        <Button size="xl-quiet" @click="importCollection">
          <Icon name="download" :size="15" :stroke-width="1.9" />
          {{ t('collections.import') }}
        </Button>
      </div>

      <!-- What the dialog will take, said before it is opened rather than by a file it refuses. -->
      <span class="start-note">{{ t('collections.importNote') }}</span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.start-page {
  /* The drawing's own lines: the window's base is Tailwind's 1.5, which leaves a 22px title taller
     than the handoff's. */
  line-height: normal;
  @apply flex-1 min-h-0 flex items-center justify-center p-10;
}

.start-card {
  @apply max-w-[460px] flex flex-col items-start gap-4;
}

.start-icon {
  @apply inline-flex items-center justify-center flex-none w-[52px] h-[52px] rounded-[14px]
         bg-bg-hover text-text-tertiary;
}

.start-text {
  @apply flex flex-col gap-2;
}

.start-title {
  @apply text-[22px] font-semibold;
  letter-spacing: -0.01em;
}

.start-body {
  @apply text-[14px] text-text-secondary;
  line-height: 1.55;
}

.start-actions {
  @apply flex items-center gap-[10px];
}

.start-note {
  @apply text-[12.5px] text-text-tertiary;
}
</style>
