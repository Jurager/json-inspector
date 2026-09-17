<script setup lang="ts">
import { computed } from 'vue'
import { Sheet, SheetRow } from '../ui/sheet'
import { useMessages } from '../../i18n'

// What a `.env` file is about to become, before it becomes it. The three lines are the whole set of
// decisions an import makes and they are the same three every time — which key wins, what counts as
// a secret, what is not read at all — so they are stated once here rather than asked per key in a
// list nobody wants to read.
const emit = defineEmits<{ (e: 'close'): void; (e: 'choose'): void }>()

const { t } = useMessages()

const rows = computed(() => [
  {
    label: t('environments.importOverwrite'),
    note: t('environments.importOverwriteNote'),
    tag: t('environments.importOn'),
    tone: 'keep' as const,
  },
  {
    label: t('environments.importSecrets'),
    note: t('environments.importSecretsNote'),
    tag: t('environments.importOn'),
    tone: 'keep' as const,
  },
  {
    label: t('environments.importComments'),
    note: t('environments.importCommentsNote'),
    tag: t('environments.importSkip'),
    tone: 'drop' as const,
  },
])
</script>

<template>
  <Sheet
    :title="t('environments.importEnv')"
    :sub="t('environments.importSub')"
    :action="t('environments.chooseFile')"
    @close="emit('close')"
    @action="emit('choose')"
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
