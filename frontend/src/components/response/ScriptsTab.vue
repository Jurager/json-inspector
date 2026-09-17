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

function count(run: ScriptRun, passed: boolean): number {
  return (run.tests ?? []).filter((test) => test.passed === passed).length
}

// What is said about the run beside its counts: which half of the request it ran around, which level
// it belongs to, and how long it took. A script is usually faster than the clock can see, and "0 мс"
// reads as a missing value rather than as a fast one — so a duration it did not measure is left out.
function caption(run: ScriptRun): string {
  const parts = [SCOPE_LABELS.value[run.scope] ?? run.scope]
  if (levelName(run)) parts.push(levelName(run))
  if (run.durationUs > 0) parts.push(formatMicros(run.durationUs))
  return parts.join(' · ')
}
</script>

<template>
  <div class="script-runs">
    <div v-if="loading" class="empty"><span>{{ t('response.scripts.loading') }}</span></div>

    <div v-else-if="runs.length === 0" class="empty">
      <span class="empty-title">{{ t('response.scripts.none') }}</span>
      <span>{{ t('response.scripts.noneHint') }}</span>
    </div>

    <template v-else>
      <section v-for="run in runs" :key="run.id" class="run">
        <!-- What the run came to, said before what it said. The failed count is the one that carries a
             colour when it is not zero: a script that broke nothing is not news. -->
        <div class="run-head">
          <template v-if="run.tests?.length">
            <span class="pill" :class="count(run, true) ? 'ok' : 'chip'">
              {{ t('response.scripts.passed', { n: count(run, true) }) }}
            </span>
            <span class="pill" :class="count(run, false) ? 'bad' : 'chip'">
              {{ t('response.scripts.failed', { n: count(run, false) }) }}
            </span>
          </template>
          <span class="run-caption">{{ caption(run) }}</span>
        </div>

        <div v-if="run.error" class="run-error mono">{{ run.error }}</div>

        <div v-if="run.tests?.length" class="tests">
          <div v-for="(test, i) in run.tests" :key="i" class="test">
            <span class="test-mark" :class="test.passed ? 'ok' : 'bad'">
              <Icon
                :name="test.passed ? 'check' : 'xmark'"
                :size="13"
                :stroke-width="3"
              />
            </span>
            <span class="test-text">
              <span class="test-name">{{ test.name }}</span>
              <span v-if="test.error" class="test-error mono">{{ test.error }}</span>
            </span>
            <span v-if="test.durationUs > 0" class="test-time mono">{{ formatMicros(test.durationUs) }}</span>
          </div>
        </div>

        <div v-if="run.logs?.length" class="console">
          <span class="console-label">{{ t('response.scripts.console') }}</span>
          <div class="console-box mono">
            <div v-for="(line, i) in run.logs" :key="i" class="console-line" :class="line.level">
              {{ line.message }}
            </div>
          </div>
        </div>

        <div v-if="!hasChecks(run)" class="run-quiet">{{ t('response.scripts.quiet') }}</div>
      </section>
    </template>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The tab is a column of blocks on the design's own 18px rhythm, the same one the timings tab stands
   on: a run's counts, its checks and its console are three blocks of that column. */
.script-runs {
  @apply min-h-full flex flex-col gap-[18px] bg-bg-panel;
  padding: 22px 24px;
}

.run {
  @apply flex flex-col gap-[18px];
}

.run-head {
  @apply flex items-center gap-3;
}

/* A count is a pill the window fills like a status: green when it is what should be, the chip when it
   is nothing, and the failure colour the moment it is not zero. */
.pill {
  @apply text-[12px] font-bold py-[5px] px-2.5 rounded-md;
}

.pill.ok {
  @apply bg-green-soft;
  color: var(--green-text);
}

.pill.bad {
  @apply bg-red-soft;
  color: var(--red-text);
}

.pill.chip {
  @apply text-text-tertiary;
  background: var(--bg-hover);
}

.run-caption {
  @apply text-[13px] text-text-tertiary;
}

.run-error {
  @apply text-[13px];
  color: var(--red-text);
}

/* The checks are a card, one row each, and the row is the height of the mark that stands in it. */
.tests {
  @apply flex flex-col border border-border rounded-xl overflow-hidden;
}

.test {
  @apply flex items-center gap-3 min-h-12 px-4 border-b border-border;
}

.test:last-child {
  border-bottom: 0;
}

.test-mark {
  @apply flex-none inline-flex items-center justify-center w-5 h-5 rounded-full;
}

.test-mark.ok {
  @apply bg-green-soft;
  color: var(--green-text);
}

.test-mark.bad {
  @apply bg-red-soft;
  color: var(--red-text);
}

/* The name and, under it, why a check failed: the reason belongs to the check and not to a console a
   level away. */
.test-text {
  @apply flex flex-col flex-1 min-w-0;
}

.test-name {
  @apply text-[14px] text-text;
}

.test-error {
  @apply text-[12.5px] break-words;
  color: var(--red-text);
}

.test-time {
  @apply flex-none text-[13px] text-text-tertiary;
}

.console {
  @apply flex flex-col gap-2;
}

.console-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.console-box {
  @apply flex flex-col border border-border rounded-xl bg-bg-inset py-3.5 px-4 text-[13.5px] leading-[1.7] text-text-secondary;
  font-family: var(--mono);
}

.console-line {
  @apply whitespace-pre-wrap break-words;
}

/* Every line a script printed starts with the design's arrow. It is drawn rather than typed: the mark
   is the console's furniture, and a line the user selects copies the line and not the mark. */
.console-line::before {
  content: '→ ';
}

.console-line.warn {
  @apply text-orange;
}

.console-line.error {
  @apply text-red;
}

.run-quiet {
  @apply text-[13px] text-text-tertiary;
}
</style>
