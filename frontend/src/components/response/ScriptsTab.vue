<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import { ScriptingService } from '../../../bindings/json-inspector/internal/transport/wails'
import { useCollectionsStore } from '../../stores/collections'
import { findCollection, findNode } from '../../lib/collectionTree'
import { formatMicros, useMessages } from '../../i18n'
import type { ScriptRun } from '../../../bindings/json-inspector/internal/domain'

const props = defineProps<{ recordId: string }>()

const store = useCollectionsStore()
const { t } = useMessages()
const runs = ref<ScriptRun[]>([])
const loading = ref(true)

// The reports of one record: read when the tab opens, and again when the record under it changes — a
// run walks request after request, and the tab follows what is selected rather than what was.
watch(
  () => props.recordId,
  async (id) => {
    loading.value = true
    runs.value = (await ScriptingService.Runs(id)) ?? []
    loading.value = false
  },
  { immediate: true }
)

// Which level a report came from. A report keeps the node id and not the node, so the name is the
// tree's answer — and a level deleted since the run has none.
function levelName(run: ScriptRun): string {
  const id = run.nodeId ?? ''
  return findNode(store.tree, id)?.name ?? findCollection(store.tree, id)?.name ?? ''
}

const SCOPE_LABELS = computed<Record<string, string>>(() => ({
  pre: t('response.scripts.pre'),
  post: t('response.scripts.post'),
}))

// A check that failed is what the tab is for: an ordinary run of a script with no checks says nothing
// here, and the line above each script is what says it ran at all.
function hasChecks(run: ScriptRun): boolean {
  return (run.tests ?? []).length > 0 || (run.logs ?? []).length > 0 || !run.ok || Boolean(run.error)
}
</script>

<template>
  <div class="script-runs">
    <div v-if="loading" class="empty"><span>{{ t('response.scripts.loading') }}</span></div>

    <div v-else-if="runs.length === 0" class="empty">
      <span class="empty-title">{{ t('response.scripts.none') }}</span>
      <span>{{ t('response.scripts.noneHint') }}</span>
    </div>

    <ul v-else class="run-list">
      <li v-for="run in runs" :key="run.id" class="run">
        <div class="run-head">
          <Icon
            class="run-icon"
            :class="run.ok ? 'ok' : 'bad'"
            :name="run.ok ? 'check' : 'xmark'"
            :size="12"
            :stroke-width="2.5"
          />
          <span class="run-scope">{{ SCOPE_LABELS[run.scope] ?? run.scope }}</span>
          <span v-if="levelName(run)" class="run-level">{{ levelName(run) }}</span>
          <!-- A script is usually faster than the clock can see, and "0 мс" reads as a missing value
               rather than as a fast one. -->
          <span v-if="run.durationUs > 0" class="run-time mono">{{ formatMicros(run.durationUs) }}</span>
        </div>

        <div v-if="run.error" class="run-error">{{ run.error }}</div>

        <ul v-if="run.tests?.length" class="checks">
          <li v-for="(test, i) in run.tests" :key="i" class="check" :class="{ failed: !test.passed }">
            <Icon
              class="check-icon"
              :class="test.passed ? 'ok' : 'bad'"
              :name="test.passed ? 'check' : 'xmark'"
              :size="11"
              :stroke-width="2.5"
            />
            <span class="check-name">{{ test.name }}</span>
            <span v-if="test.error" class="check-error">{{ test.error }}</span>
          </li>
        </ul>

        <ul v-if="run.logs?.length" class="logs">
          <li v-for="(line, i) in run.logs" :key="i" class="log mono" :class="line.level">
            {{ line.message }}
          </li>
        </ul>

        <div v-if="!hasChecks(run)" class="run-quiet">{{ t('response.scripts.quiet') }}</div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.script-runs {
  @apply min-h-full flex flex-col bg-bg-panel;
}

.run-list {
  @apply flex flex-col px-4 py-3 gap-3;
}

.run {
  @apply flex flex-col gap-1.5 pb-3;
}

.run:not(:last-child) {
  @apply border-b border-border;
}

.run-head {
  @apply flex items-center gap-2 text-[12.5px];
}

.run-icon.ok,
.check-icon.ok {
  @apply text-green;
}

.run-icon.bad,
.check-icon.bad {
  @apply text-red;
}

.run-scope {
  @apply font-medium;
}

.run-level {
  @apply text-text-tertiary;
}

.run-time {
  @apply ml-auto text-[11.5px] text-text-tertiary;
}

.run-error {
  @apply text-[12px] text-red;
}

.run-quiet {
  @apply text-[12px] text-text-tertiary;
}

.checks {
  @apply flex flex-col gap-1;
}

.check {
  @apply flex items-baseline gap-2 text-[12.5px];
}

.check.failed {
  @apply text-red;
}

.check-name {
  @apply flex-none;
}

.check-error {
  @apply flex-1 min-w-0 text-[11.5px] text-text-secondary;
}

.logs {
  @apply flex flex-col gap-0.5;
}

.log {
  @apply text-[11.5px] text-text-secondary whitespace-pre-wrap break-words;
}

.log.warn {
  @apply text-orange;
}

.log.error {
  @apply text-red;
}
</style>
