<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from '../ui/popover'
import { copyToClipboard } from '../../lib/clipboard'
import type { RecordView } from '../../lib/requestRecord'
import { useMessages } from '../../i18n'

// The line a captured request gets, and the only one it can: it has no command line to be read in,
// so where it went and what it carried are said here. What came back is the strip below — a method
// and a status in this line would be the response said twice.
//
// The field is the editor's own object, drawn read-only: the verb standing in its left end, the
// address after it. There is no way back from here; the sidebar is where the records are chosen.
const props = defineProps<{ record: RecordView }>()

const { t } = useMessages()

// The host and the path of the address, without the query: the query is what the Params button
// beside it is for, and an address spelled twice reads as two different addresses.
function hostPath(url: string): string {
  try {
    const parsed = new URL(url)
    return parsed.host + parsed.pathname
  } catch {
    return url
  }
}

// The verb's plate: the same object the editor draws, in the two colours the drawing gives it — a
// read is green, and everything else is the accent.
const methodPlate = computed(() =>
  props.record.method === 'GET'
    ? { color: 'var(--green-text)', background: 'var(--green-soft)' }
    : { color: 'var(--accent)', background: 'var(--accent-soft)' }
)

// Values with commas are list-style JSON:API params (include, fields[type]) — chipped one item at a
// time rather than read as one long string.
const params = computed(() => {
  const out: { name: string; values: string[] }[] = []
  try {
    new URL(props.record.url).searchParams.forEach((value, name) =>
      out.push({ name, values: value.split(',') })
    )
  } catch {
    // Not an address: there is no query to draw, and the field above shows it as it came.
  }
  return out
})

// The same query as one line of text, which is what a reader copies.
const query = computed(() => {
  try {
    return new URL(props.record.url).search
  } catch {
    return ''
  }
})

// A number reads as a number and a word as a word, which is the whole of what the tint says.
function tint(value: string): string {
  return /^-?\d+(\.\d+)?$/.test(value.trim()) ? 'num' : ''
}

const copied = ref(false)

async function copyQuery() {
  if (!(await copyToClipboard(query.value))) return
  copied.value = true
  setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="request-block request-block-solo">
    <div class="bar-row">
      <div class="url-field">
        <span class="method-plate" :style="methodPlate">{{ record.method }}</span>
        <span class="req-url mono" :title="record.url">{{ hostPath(record.url) }}</span>
      </div>

      <div v-if="params.length" class="segments">
        <Popover>
          <PopoverTrigger as-child>
            <button class="segment filled">
              <span>{{ t('request.chips.params') }}</span>
              <span class="segment-count">{{ params.length }}</span>
            </button>
          </PopoverTrigger>
          <!-- Hung from the button's right edge rather than its left: the button is at the end of
               the bar, and a 620px panel opening to the right of it would run off the window. -->
          <PopoverContent class="params-menu" align="end" :side-offset="9">
            <div class="params-head">
              <span class="params-title">{{ t('response.paramsTitle') }}</span>
              <span class="params-count mono">{{ params.length }}</span>
              <span class="params-spacer"></span>
              <button class="params-copy" @click="copyQuery">
                {{ copied ? t('common.copied') : t('response.copyQuery') }}
              </button>
              <PopoverClose as-child>
                <IconButton :hint="t('common.close')" size="xl">
                  <Icon name="xmark" :size="14" />
                </IconButton>
              </PopoverClose>
            </div>

            <!-- One row per parameter rather than one grid with rows in it: the divider between two
                 parameters is drawn by the row and spans it, which a cell of a shared grid cannot do. -->
            <div v-for="p in params" :key="p.name" class="params-row">
              <div class="params-name-cell">
                <span class="params-name mono">{{ p.name }}</span>
                <span v-if="p.values.length > 1" class="params-item-count mono">
                  {{ p.values.length }}
                </span>
              </div>
              <div class="params-chips">
                <span v-for="(v, i) in p.values" :key="i" class="params-chip mono" :class="tint(v)">
                  {{ v }}
                </span>
              </div>
            </div>

            <!-- The query as one string, which is what the button above hands over: a chip per value
                 says what the request asked for, and this says what the address was. -->
            <div class="params-string">
              <span class="params-string-label">{{ t('response.queryString') }}</span>
              <span class="params-string-text mono">{{ query }}</span>
            </div>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The field itself, the verb's plate and the sections are the shared shapes in style.css — the same
   objects the editor's bar draws. What is this component's own is the address inside the field, and
   the panel the Params button opens. */
.req-url {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-text;
  font-family: var(--mono);
  font-size: 13px;
}

.params-head {
  @apply flex items-center gap-2.5 pb-2.5;
}

.params-title {
  @apply text-sm font-semibold;
}

.params-count {
  @apply text-[13px] text-text-tertiary;
  font-family: var(--mono);
}

.params-spacer {
  @apply flex-1;
}

.params-copy {
  @apply flex-none h-7 px-2.5 rounded-[7px] border cursor-pointer text-[13px] text-text
         bg-bg-inset border-border-strong;
  font-family: inherit;
  --wails-draggable: no-drag;
}

.params-copy:hover {
  @apply bg-bg-hover;
}

.params-row {
  @apply grid items-start gap-3 py-2.5 px-1.5;
  grid-template-columns: 170px minmax(0, 1fr);
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.params-name-cell {
  @apply flex items-baseline gap-[7px];
}

.params-name {
  @apply text-[13px] text-text;
}

.params-item-count {
  @apply text-[12px] text-text-tertiary;
}

.params-chips {
  @apply flex flex-wrap gap-[5px];
}

.params-chip {
  @apply text-[13px] text-accent py-[3px] px-2 rounded-md;
  background: var(--accent-soft);
}

/* A number is not a word, and the pair of colours is what says which of the two a value is. */
.params-chip.num {
  @apply text-tok-num bg-bg-hover;
}

/* The query as one string: what the chips say value by value, in the shape it is pasted in. */
.params-string {
  @apply flex gap-3 mt-2.5 p-3 rounded-[8px] bg-bg-inset;
}

.params-string-label {
  @apply flex-none pt-[2px] text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.params-string-text {
  @apply flex-1 min-w-0 text-[13px] break-all;
  line-height: 1.6;
  color: var(--text-secondary);
}
</style>
