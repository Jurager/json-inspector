<script setup lang="ts">
import { computed } from 'vue'
import type { RecordView } from '../../lib/requestRecord'
import { formatMicros } from '../../lib/format'

const props = defineProps<{ record: RecordView }>()

interface Phase {
  label: string
  us: number
}

// Only the phases that happened are drawn, and which those are is Go's answer: a phase is absent when
// it did not take place at all — a repeat to a server the app is already talking to dials nothing, and
// a captured request carries only the two phases a page can time. A zero would be a different
// statement: a phase that was measured and took no measurable time.
const phases = computed<Phase[]>(() =>
  (
    [
      { label: 'DNS', us: props.record.dnsUs },
      { label: 'TCP', us: props.record.connectUs },
      { label: 'TLS', us: props.record.tlsUs },
      { label: 'Ожидание', us: props.record.waitUs },
      { label: 'Загрузка', us: props.record.downloadUs },
    ] as { label: string; us: number | null | undefined }[]
  )
    .filter((phase): phase is Phase => phase.us != null)
    .map((phase) => ({ label: phase.label, us: phase.us }))
)

const maxUs = computed(() => Math.max(1, ...phases.value.map((p) => p.us)))
</script>

<template>
  <div class="timings">
    <div class="timing-row timing-total">
      <span class="timing-label">Всего</span>
      <div class="timing-track">
        <div class="timing-fill" style="width: 100%"></div>
      </div>
      <span class="timing-value mono">{{ formatMicros(record.durationUs) }}</span>
    </div>
    <div v-for="p in phases" :key="p.label" class="timing-row">
      <span class="timing-label">{{ p.label }}</span>
      <div class="timing-track">
        <div class="timing-fill" :style="{ width: (p.us / maxUs) * 100 + '%' }"></div>
      </div>
      <span class="timing-value mono">{{ formatMicros(p.us) }}</span>
    </div>
    <div v-if="phases.length === 0" class="timing-note">
      Запрос не удалось засечь по фазам — ответа не было.
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.timings {
  @apply p-4.5 min-h-full flex flex-col gap-3 bg-bg-panel;
}

.timing-row {
  @apply grid grid-cols-[130px_1fr_64px] gap-3 items-center;
}

.timing-label {
  @apply text-xs text-text-secondary;
}

.timing-track {
  @apply h-2 rounded-full bg-bg-inset overflow-hidden;
}

.timing-fill {
  @apply h-full rounded-full bg-accent;
}

.timing-value {
  @apply text-xs text-text text-right;
  font-variant-numeric: tabular-nums;
}

.timing-note {
  @apply text-xs text-text-tertiary;
}

.timing-total .timing-label {
  @apply font-medium text-text;
}
</style>
