<script setup lang="ts">
import { ref } from 'vue'
import ScriptsFields from '../request/ScriptsFields.vue'
import { useMessages } from '../../i18n'
import { useCollectionsStore } from '../../stores/collections'

// The code a level runs around its requests, in the sheet's own shape: one half at a time behind a
// switch, saved by the footer's button rather than while it is typed.
const { t } = useMessages()

const store = useCollectionsStore()

const fields = ref<{ commit: () => Promise<void>; discard: () => void } | null>(null)

// The footer's buttons belong to the sheet and the code belongs to the fields, so the two are handed
// up rather than handled here.
const commit = async () => {
  await fields.value?.commit()
}

function discard() {
  fields.value?.discard()
}

defineExpose({ commit, discard })
</script>

<template>
  <div class="scripts">
    <ScriptsFields ref="fields" :source="store" :note="t('collections.scriptsNote')" />
  </div>
</template>

<style scoped>
@reference "../../style.css";

.scripts {
  @apply flex flex-col;
}
</style>
