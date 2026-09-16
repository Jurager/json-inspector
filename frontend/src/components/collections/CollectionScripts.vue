<script setup lang="ts">
import ScriptsFields from '../request/ScriptsFields.vue'
import { useMessages } from '../../i18n'
import { useCollectionsStore } from '../../stores/collections'

const { t } = useMessages()

const store = useCollectionsStore()
</script>

<template>
  <div class="scripts">
    <ScriptsFields
      :source="store"
      :note="t('collections.scriptsNote')"
    />
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The tab's own room; the boxes are the same two the chip draws, with more space around them. */
.scripts {
  @apply max-w-[620px] px-6 py-6;
}
</style>
