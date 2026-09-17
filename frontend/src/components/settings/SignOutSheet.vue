<script setup lang="ts">
import { computed } from 'vue'
import { Sheet, SheetRow } from '../ui/sheet'
import { useAccount } from '../../composables/useAccount'
import { useMessages } from '../../i18n'

// What leaving the account costs, asked before it happens. It is the same list the drawing puts over
// the window, and the same one in both places a person can leave from — the account category and the
// menu in the rail — because a confirmation that differs by the door is a confirmation nobody read.
//
// Every row here is true of this build: nothing is synced yet, so nothing is taken away with the
// account, and the words say that rather than promising a keychain or a team that do not exist.
const emit = defineEmits<{ (e: 'close'): void; (e: 'confirm'): void }>()

const { state } = useAccount()
const { t } = useMessages()

const account = computed(() => state.value?.account ?? null)

const sub = computed(() => [account.value?.email, account.value?.server].filter(Boolean).join(' · '))

const rows = computed(() => [
  {
    label: t('account.signOutSheetDevice'),
    note: t('account.signOutSheetDeviceNote'),
    tag: t('account.signOutSheetDeviceTag'),
    tone: 'drop' as const,
  },
  {
    label: t('account.signOutSheetData'),
    note: t('account.signOutSheetDataNote'),
    tag: t('common.kept'),
    tone: 'keep' as const,
  },
  {
    label: t('account.signOutSheetSecrets'),
    note: t('account.signOutSheetSecretsNote'),
    tag: t('common.kept'),
    tone: 'keep' as const,
  },
])
</script>

<template>
  <Sheet
    :title="t('account.signOut')"
    :sub="sub"
    :cancel="t('common.cancel')"
    :action="t('account.signOut')"
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
