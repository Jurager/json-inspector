<script setup lang="ts">
import { computed } from 'vue'
import EmptyPage from '../ui/EmptyPage.vue'
import { Button } from '../ui/button'
import Icon from '../ui/Icon.vue'
import { useCollectionsStore } from '../../stores/collections'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { describeFailure, useMessages } from '../../i18n'
import { useToast } from '../../composables/useToast'

// Until the first collection exists there is nothing to list, so the window says what a collection is,
// what it is made of and how one is started — the panel with a lone «+» would say less.
const store = useCollectionsStore()
const workspaces = useWorkspacesStore()

const { t } = useMessages()
const toast = useToast()

// The page names the space it is about: a window can hold several of them, and «no collections» is a
// sentence that needs its subject when the one on screen is not the only one there is. The default
// space carries no name of its own, so the interface's word for it is what the sentence gets.
const space = computed(() => (workspaces.active ? workspaceName(workspaces.active) : ''))

// The three steps of the drawing, in the app's own words: a collection is a folder with its own
// variables and authorization, a run of it is a report — and both of those are what the section is
// for, not a promise of something that is not there.
const steps = computed(() => [
  { n: '1', title: t('collections.stepCreate'), body: t('collections.stepCreateBody') },
  { n: '2', title: t('collections.stepAdd'), body: t('collections.stepAddBody') },
  { n: '3', title: t('collections.stepRun'), body: t('collections.stepRunBody') },
])

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
  <EmptyPage
    :badge="t('collections.emptyBadge', { name: space })"
    :title="t('collections.emptyHead')"
    :body="t('collections.emptyLede')"
    :steps="steps"
  >
    <template #actions>
      <Button variant="primary" size="xl" @click="create">
        <Icon name="plus" :size="15" :stroke-width="2.2" />
        {{ t('collections.newCollection') }}
      </Button>
      <Button size="xl-quiet" @click="importCollection">
        <Icon name="download" :size="15" :stroke-width="1.9" />
        {{ t('collections.import') }}
      </Button>
    </template>

    <!-- What the dialog will take, said before it is opened rather than by a file it refuses. -->
    <template #note>{{ t('collections.importNote') }}</template>
  </EmptyPage>
</template>
