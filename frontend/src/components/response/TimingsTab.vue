<script setup lang="ts">
import { computed } from 'vue'
import type { RecordView } from '../../lib/requestRecord'
import { formatMicros, useMessages } from '../../i18n'

const props = defineProps<{ record: RecordView }>()
const { t } = useMessages()

interface Phase {
  label: string
  us: number
  // The wait is the one phase the design colours: it is the one the server is answerable for, and the
  // one a person comparing two attempts is looking at.
  accent: boolean
}

// Only the phases that happened are drawn, and which those are is Go's answer. An absent phase is one
// that did not take place at all — a zero would say it was measured and took no time. A request sent
// with connection reuse has no DNS, TCP or TLS because nothing was dialled; a captured one carries
// only the two phases a page can time.
const phases = computed<Phase[]>(() =>
  (
    [
      { label: t('response.timings.dns'), us: props.record.dnsUs, accent: false },
      { label: t('response.timings.connect'), us: props.record.connectUs, accent: false },
      { label: t('response.timings.tls'), us: props.record.tlsUs, accent: false },
      { label: t('response.timings.wait'), us: props.record.waitUs, accent: true },
      { label: t('response.timings.download'), us: props.record.downloadUs, accent: false },
    ] as { label: string; us: number | null | undefined; accent: boolean }[]
  )
    .filter((phase): phase is Phase => phase.us != null)
    .map((phase) => ({ label: phase.label, us: phase.us, accent: phase.accent }))
)

// The line under the total. The design's sentence names the wait inside it, because that is the part
// of a request the server owns; where nothing was timed waiting, the sentence stops at the total.
const totalNote = computed(() => {
  const wait = props.record.waitUs
  if (wait == null) return t('response.timings.total')
  return t('response.timings.totalNote', { wait: formatMicros(wait) })
})

// Every bar is measured against the whole request, because that is what the "Всего" bar above them
// is: the phases are its parts, and a part drawn against the largest of the others claims to be all
// of it. The phases add up to the total, so the bars do too.
const totalUs = computed(() => Math.max(1, props.record.durationUs))

function width(us: number): string {
  return `${Math.min(100, (us / totalUs.value) * 100)}%`
}

// A request that reused a connection has no dial phases to show. The connect phase is what tells the
// two apart — a fresh connection always has one, a reused one never does, and a request that never
// reached a server has neither it nor a wait, so it is already answered for by the note above.
const reusedConnection = computed(
  () =>
    props.record.connectUs == null &&
    props.record.source === 'manual' &&
    (props.record.waitUs != null || props.record.downloadUs != null)
)

</script>

<template>
  <div class="timings">
    <!-- The total is a headline and not a bar: the phases below are its parts and each is drawn
         against it, so a bar of its own length would be the one bar saying nothing. -->
    <div class="head">
      <span class="total">{{ formatMicros(record.durationUs) }}</span>
      <span class="caption">{{ totalNote }}</span>
    </div>
    <div class="bars">
      <div v-for="p in phases" :key="p.label" class="timing-row">
        <span class="timing-label">{{ p.label }}</span>
        <div class="timing-track">
          <div class="timing-fill" :class="{ wait: p.accent }" :style="{ width: width(p.us) }"></div>
        </div>
        <span class="timing-value mono">{{ formatMicros(p.us) }}</span>
      </div>
    </div>
    <div v-if="phases.length === 0" class="timing-note">
      {{ t('response.timings.unmeasured') }}
    </div>
    <div v-else-if="reusedConnection" class="timing-note">
      {{ t('response.timings.reused') }}
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.timings {
  @apply min-h-full flex flex-col gap-[18px] bg-bg-panel;
  padding: 22px 24px;
}

.head {
  @apply flex items-baseline gap-3;
}

.total {
  @apply text-[26px] font-semibold;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.caption {
  @apply text-[13px] text-text-tertiary;
}

.bars {
  @apply flex flex-col gap-3.5;
}

.timing-row {
  @apply grid grid-cols-[170px_minmax(0,1fr)_80px] gap-4 items-center;
}

.timing-label {
  @apply text-[13px] text-text-secondary;
}

/* The track is the chip's own fill and the phases are drawn in the grey of a caption: the design
   keeps one hue for everything a request spent on its own and the accent for the wait, so that the
   bar the eye lands on is the one the server is answerable for. */
.timing-track {
  @apply h-2.5 rounded-full bg-bg-hover overflow-hidden;
}

.timing-fill {
  @apply h-full rounded-full;
  background: var(--text-tertiary);
}

.timing-fill.wait {
  @apply bg-accent;
}

.timing-value {
  @apply text-[13px] text-text text-right;
  font-variant-numeric: tabular-nums;
}

.timing-note {
  @apply text-[13px] text-text-tertiary;
}
</style>
