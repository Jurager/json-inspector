<script setup lang="ts">
import { computed } from 'vue'
import { Sheet, SheetRow } from '../ui/sheet'
import { useWorkspacesStore } from '../../stores/workspaces'
import { useMessages } from '../../i18n'

// What is about to go, before it goes. A workspace takes everything in it — and what it does *not*
// take is worth saying in the same breath: the theme, the language, the order of the panels and how
// long history is kept belong to the installation, not to a space, and the list says so rather than
// leaving the reader to wonder.
const emit = defineEmits<{ (e: 'close'): void; (e: 'confirm'): void }>()

const store = useWorkspacesStore()
const { t } = useMessages()

const held = computed(() => (store.edited ? store.countsOf(store.edited.id) : null))

const rows = computed(() => [
  {
    label: t('counts.collections', held.value?.collections ?? 0),
    note: t('workspaces.deleteSheetCollectionsNote'),
    tag: t('common.deleted'),
    tone: 'drop' as const,
  },
  {
    label: t('counts.environments', held.value?.environments ?? 0),
    note: t('workspaces.deleteSheetEnvironmentsNote'),
    tag: t('common.deleted'),
    tone: 'drop' as const,
  },
  {
    label: t('counts.runs', held.value?.runs ?? 0),
    note: t('workspaces.deleteSheetRunsNote'),
    tag: t('common.deleted'),
    tone: 'drop' as const,
  },
  {
    label: t('workspaces.deleteSheetSettings'),
    note: t('workspaces.deleteSheetSettingsNote'),
    tag: t('common.kept'),
    tone: 'keep' as const,
  },
])

const sub = computed(() => t('workspaces.deleteSub'))
</script>

<template>
  <Sheet
    :title="t('workspaces.deleteTitle')"
    :sub="sub"
    :cancel="t('common.cancel')"
    :action="t('workspaces.removeAction')"
    @close="emit('close')"
    @cancel="emit('close')"
    @action="emit('confirm')"
  >
    <div class="rows">
      <SheetRow
        v-for="row in rows"
        :key="row.label"
        :label="row.label"
        :note="row.note"
        :tag="row.tag"
        :tone="row.tone"
      />
    </div>
  </Sheet>
</template>

<style scoped>
@reference "../../style.css";

.rows {
  @apply flex flex-col gap-0.5;
}
</style>
