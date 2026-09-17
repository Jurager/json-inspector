<script setup lang="ts">
import { computed } from 'vue'
import { Sheet, SheetRow } from '../ui/sheet'
import { useEnvironmentsStore } from '../../stores/environments'
import { useMessages } from '../../i18n'

// What is about to go, before it goes. The two destructive things in this window are one shape: an
// environment takes its variables and their secrets with it, and the globals — which cannot be
// deleted, only emptied — take what is written in them and leave every environment alone.
const props = defineProps<{ kind: 'delete' | 'clear'; referenced: boolean }>()

const emit = defineEmits<{ (e: 'close'): void; (e: 'confirm'): void }>()

const envStore = useEnvironmentsStore()
const { t } = useMessages()

const env = computed(() => envStore.environments.find((e) => e.id === envStore.editedEnvId) ?? null)
const secretCount = computed(() => env.value?.vars.filter((v) => v.kind === 'secret').length ?? 0)

const rows = computed(() =>
  props.kind === 'delete'
    ? [
        {
          label: t('environments.deleteSheetVariables'),
          note: t('environments.deleteSheetVariablesNote'),
          tag: t('common.deleted'),
          tone: 'drop' as const,
        },
        {
          label: t('environments.deleteSheetSecrets', { n: secretCount.value }),
          note: t('environments.deleteSheetSecretsNote'),
          tag: t('common.deleted'),
          tone: 'drop' as const,
        },
        {
          label: t('environments.globals'),
          note: t('environments.deleteSheetGlobalsNote'),
          tag: t('common.kept'),
          tone: 'keep' as const,
        },
      ]
    : [
        {
          label: t('environments.clearSheetVars', { n: envStore.globals.length }),
          note: t('environments.deleteSheetVariablesNote'),
          tag: t('common.deleted'),
          tone: 'drop' as const,
        },
        {
          label: t('environments.clearSheetOverrides'),
          note: t('environments.clearSheetOverridesNote'),
          tag: t('common.kept'),
          tone: 'keep' as const,
        },
      ]
)

// Nothing references an environment until something does: the sentence the drawing writes is about
// requests that use the variables, and a plain one is the truth when there are none.
const sub = computed(() => {
  if (props.kind === 'clear') return t('environments.clearSheetSub')
  return props.referenced ? t('environments.deleteSheetSub') : t('environments.deleteSheetSubPlain')
})
</script>

<template>
  <Sheet
    :title="kind === 'delete' ? t('environments.dangerDelete') : t('environments.dangerClear')"
    :sub="sub"
    :cancel="t('common.cancel')"
    :action="kind === 'delete' ? t('environments.sheetDelete') : t('environments.sheetClear')"
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
