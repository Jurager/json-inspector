<script setup lang="ts">
import { computed } from 'vue'
import type { RequestRecord } from '../../lib/types'

const props = defineProps<{ record: RequestRecord }>()

interface Phase {
  label: string
  ms: number
}

// Phases only exist for requests made from the app itself (httptrace); the
// extension reports just a total duration.
const phases = computed<Phase[]>(() => [
  { label: 'DNS', ms: props.record.dnsMs ?? 0 },
  { label: 'TCP', ms: props.record.connectMs ?? 0 },
  { label: 'TLS', ms: props.record.tlsMs ?? 0 },
  { label: 'Ожидание', ms: props.record.waitMs ?? 0 },
  { label: 'Загрузка', ms: props.record.downloadMs ?? 0 },
])

const hasDetail = computed(() => props.record.source === 'manual')

const maxMs = computed(() => Math.max(1, ...phases.value.map((p) => p.ms)))
</script>

<template>
  <div class="timings">
    <template v-if="hasDetail">
      <div class="timing-row timing-total">
        <span class="timing-label">Всего</span>
        <div class="timing-track">
          <div class="timing-fill" style="width: 100%"></div>
        </div>
        <span class="timing-value mono">{{ record.durationMs }} мс</span>
      </div>
      <div v-for="p in phases" :key="p.label" class="timing-row">
        <span class="timing-label">{{ p.label }}</span>
        <div class="timing-track">
          <div class="timing-fill" :style="{ width: (p.ms / maxMs) * 100 + '%' }"></div>
        </div>
        <span class="timing-value mono">{{ p.ms }} мс</span>
      </div>
    </template>
    <template v-else>
      <div class="timing-row">
        <span class="timing-label">Всего</span>
        <div class="timing-track">
          <div class="timing-fill" style="width: 100%"></div>
        </div>
        <span class="timing-value mono">{{ record.durationMs }} мс</span>
      </div>
      <div class="timing-note">Детализация доступна только для запросов из приложения</div>
    </template>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.timings {
  @apply p-4.5 flex flex-col gap-3;
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
  @apply text-[11.5px] text-text text-right;
  font-variant-numeric: tabular-nums;
}

.timing-note {
  @apply text-[11.5px] text-text-tertiary;
}

.timing-total .timing-label {
  @apply font-medium text-text;
}
</style>
