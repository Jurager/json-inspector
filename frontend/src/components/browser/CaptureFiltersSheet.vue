<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Checkbox } from '../ui/checkbox'
import { Input } from '../ui/input'
import { Sheet, SheetRow } from '../ui/sheet'
import { useSettings } from '../../composables/useSettings'
import { useToast } from '../../composables/useToast'
import { describeFailure, useMessages } from '../../i18n'
import type { CaptureFilters } from '../../../bindings/json-inspector/internal/domain'

// The rules the extension filters by, in the one sheet both windows raise: the browser page from its
// card, and the settings window from the row about captured traffic. Everything the sheet needs is
// here — the draft, the hosts line, the three switches and the write — because a second copy of it
// would be a second place for the same rules to disagree.
//
// The rules are the sheet's while it is open and Save is what writes them: a switch that wrote on
// every click would leave Cancel with nothing to cancel.
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const { settings, setCaptureFilters } = useSettings()
const toast = useToast()
const { t } = useMessages()

const draft = ref<CaptureFilters>(empty())
const hostsText = ref('')
// A write that fails says so here rather than only in a toast: the sheet is raised by two windows and
// one of them — the settings window — is a document with nowhere to put a toast.
const notice = ref('')

// Three of the rules are the same kind of switch, so they are one list rather than three blocks of
// markup that have to be kept in step by hand.
const rules = computed(() => [
  {
    name: t('browser.ruleStatic'),
    note: t('browser.ruleStaticNote'),
    on: () => draft.value.static,
    toggle: () => (draft.value.static = !draft.value.static),
  },
  {
    name: t('browser.ruleAnalytics'),
    note: t('browser.ruleAnalyticsNote'),
    on: () => draft.value.analytics,
    toggle: () => (draft.value.analytics = !draft.value.analytics),
  },
  {
    name: t('browser.ruleJson'),
    note: t('browser.ruleJsonNote'),
    on: () => draft.value.json,
    toggle: () => (draft.value.json = !draft.value.json),
  },
])

function empty(): CaptureFilters {
  return { hosts: [], static: true, analytics: false, json: false }
}

onMounted(() => {
  const current = settings.value?.captureFilters ?? empty()
  draft.value = { ...current, hosts: [...(current.hosts ?? [])] }
  hostsText.value = (current.hosts ?? []).join(', ')
})

async function save() {
  const hosts = hostsText.value
    .split(',')
    .map((host) => host.trim())
    .filter(Boolean)

  notice.value = ''
  try {
    await setCaptureFilters({ ...draft.value, hosts })
  } catch (error) {
    notice.value = describeFailure(error)
    return
  }
  toast.show(t('browser.filtersSaved'))
  emit('saved')
}
</script>

<template>
  <Sheet
    :title="t('browser.filtersSheet')"
    :sub="t('browser.filtersSub')"
    :cancel="t('common.cancel')"
    :action="t('browser.filtersSave')"
    @close="emit('close')"
    @cancel="emit('close')"
    @action="save"
  >
    <div class="rows">
      <SheetRow :label="t('browser.ruleHosts')" :note="t('browser.ruleHostsNote')">
        <Input
          v-model="hostsText"
          mono
          :placeholder="t('browser.ruleHostsPlaceholder')"
          spellcheck="false"
        />
      </SheetRow>

      <SheetRow v-for="rule in rules" :key="rule.name" :label="rule.name" :note="rule.note">
        <Checkbox :model-value="rule.on()" @update:model-value="rule.toggle()" />
      </SheetRow>

      <span v-if="notice" class="notice">{{ notice }}</span>
    </div>
  </Sheet>
</template>

<style scoped>
@reference "../../style.css";

.rows {
  @apply flex flex-col gap-0.5;
}

/* The hosts line has one width wherever the sheet is raised, so a list of hosts does not drag the
   row's other half around as it grows. */
.rows :deep(input) {
  @apply flex-none w-[180px];
}

.notice {
  @apply text-[13px];
  padding: 6px 10px 0;
  color: var(--red-text);
}
</style>
