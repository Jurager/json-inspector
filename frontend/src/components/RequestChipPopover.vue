<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import Icon from './Icon.vue'
import { useRequestsStore } from '../stores/requests'

const props = defineProps<{ chip: 'params' | 'headers' | 'auth' | 'body' }>()

const store = useRequestsStore()

const title = computed(() => {
  switch (props.chip) {
    case 'params':
      return 'Параметры запроса'
    case 'headers':
      return 'Заголовки'
    case 'auth':
      return 'Авторизация'
    case 'body':
      return 'Тело запроса'
  }
})

const isBodyDisabled = computed(() => store.draft.method === 'GET' || store.draft.method === 'HEAD')

const AUTH_TYPES = ['none', 'bearer', 'basic', 'oauth2'] as const
const AUTH_LABELS: Record<string, string> = { none: 'Нет', bearer: 'Bearer', basic: 'Basic', oauth2: 'OAuth 2' }

// A sliding pill behind the segments: the indicator moves one segment (+ the
// 2px gap) per step, animated by CSS transition on transform.
const activeAuthIndex = computed(() => AUTH_TYPES.indexOf(store.draft.auth.type))
const authIndicatorStyle = computed(() => ({
  transform: `translateX(calc(${activeAuthIndex.value} * (100% + 2px)))`,
}))

function close() {
  store.setOpenChip(null)
}

// Value colour hints at its type — numbers in --tok-num, everything else as a
// string in --tok-str — mirroring the JSON tree's value highlighting.
function valueClass(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : 'str'
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="chip-popover" :class="props.chip === 'auth' ? 'w-[380px]' : 'w-[460px]'">
    <div class="popover-head">
      <span class="popover-title">{{ title }}</span>
      <button class="popover-close" title="Закрыть (Esc)" @click="close"><Icon name="xmark" :size="14" /></button>
    </div>

    <!-- params / headers share the same row grid -->
    <template v-if="props.chip === 'params' || props.chip === 'headers'">
      <div class="row-head">
        <span></span>
        <span class="col-label mono">Имя</span>
        <span class="col-label mono">Значение</span>
        <span></span>
      </div>

      <template v-if="props.chip === 'params'">
        <div v-for="(p, i) in store.draft.params" :key="i" class="row" :class="{ off: !p.enabled }">
          <button class="row-check" :class="{ on: p.enabled }" @click.stop="store.toggleParam(i)">
            <Icon v-if="p.enabled" name="check" :size="10" />
          </button>
          <input :value="p.name" class="row-input mono" placeholder="имя" spellcheck="false" @input="store.updateParam(i, { name: ($event.target as HTMLInputElement).value })" />
          <input :value="p.value" class="row-input mono" :class="valueClass(p.value)" placeholder="значение" spellcheck="false" @input="store.updateParam(i, { value: ($event.target as HTMLInputElement).value })" />
          <button class="row-del" title="Удалить" @click.stop="store.removeParam(i)"><Icon name="xmark" :size="12" /></button>
        </div>
        <div class="popover-foot">
          <button class="add-btn" @click="store.addParam()">+ Параметр</button>
          <span class="foot-hint">Выключенные не уходят в запрос</span>
        </div>
      </template>

      <template v-else>
        <div v-for="(h, i) in store.draft.headers" :key="i" class="row" :class="{ off: !h.enabled }">
          <button class="row-check" :class="{ on: h.enabled }" @click.stop="store.toggleHeader(i)">
            <Icon v-if="h.enabled" name="check" :size="10" />
          </button>
          <input :value="h.name" class="row-input mono" placeholder="Header" spellcheck="false" @input="store.updateHeader(i, { name: ($event.target as HTMLInputElement).value })" />
          <input :value="h.value" class="row-input mono" :class="valueClass(h.value)" placeholder="Value" spellcheck="false" @input="store.updateHeader(i, { value: ($event.target as HTMLInputElement).value })" />
          <button class="row-del" title="Удалить" @click.stop="store.removeHeader(i)"><Icon name="xmark" :size="12" /></button>
        </div>
        <div class="popover-foot">
          <button class="add-btn" @click="store.addHeader()">+ Заголовок</button>
          <span class="foot-hint">Accept подставлен по умолчанию</span>
        </div>
      </template>
    </template>

    <template v-else-if="props.chip === 'auth'">
      <div class="segmented">
        <span class="seg-indicator" :style="authIndicatorStyle"></span>
        <button
          v-for="t in AUTH_TYPES"
          :key="t"
          class="seg"
          :class="{ active: store.draft.auth.type === t }"
          @click="store.draft.auth.type = t"
        >
          {{ AUTH_LABELS[t] }}
        </button>
      </div>
      <input
        v-if="store.draft.auth.type !== 'none'"
        v-model="store.draft.auth.token"
        class="row-input mono"
        placeholder="Токен"
        spellcheck="false"
      />
      <div class="foot-hint">Значение можно взять из окружения — переменные подставляются в URL, заголовки и тело.</div>
    </template>

    <template v-else>
      <textarea
        v-if="!isBodyDisabled"
        v-model="store.draft.body"
        class="body-area mono"
        placeholder="{ ... JSON body ... }"
        spellcheck="false"
      ></textarea>
      <div v-else class="body-disabled">{{ store.draft.method }} не отправляет тело</div>
    </template>
  </div>
</template>

<style scoped>
@reference "../style.css";

.chip-popover {
  @apply absolute top-[calc(100%+6px)] right-3 z-40 rounded-[10px] p-2.5 flex flex-col gap-2;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.16);
}

.popover-head {
  @apply flex items-center justify-between px-1 pb-2;
}

.popover-title {
  @apply text-xs font-semibold;
}

.popover-close {
  @apply w-6 h-6 border-none bg-transparent text-text-tertiary rounded-md cursor-pointer flex items-center justify-center;
}

.popover-close:hover {
  @apply bg-bg-hover text-text;
}

.row-head {
  @apply grid grid-cols-[20px_150px_1fr_22px] gap-1.5 px-1 pb-0.5;
}

.col-label {
  @apply text-[10px] uppercase tracking-wider text-text-tertiary;
}

.row {
  @apply grid grid-cols-[20px_150px_1fr_22px] gap-1.5 items-center py-[3px] px-1 rounded-md;
}

.row.off {
  @apply opacity-55;
}

.row-check {
  @apply w-[13px] h-[13px] rounded-[3px] border border-border-strong bg-bg-panel flex items-center justify-center cursor-pointer text-white;
}

.row-check.on {
  @apply bg-accent border-accent;
}

.row-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-[11.5px] px-1 py-0.5 rounded-sm;
  font-family: var(--mono);
  color: var(--text);
}

.row-input:focus {
  background: var(--bg-inset);
}

.row-input.str {
  color: var(--tok-str);
}

.row-input.num {
  color: var(--tok-num);
}

.row-del {
  @apply w-[22px] h-[22px] border-none bg-transparent text-text-tertiary rounded-md cursor-pointer flex items-center justify-center;
}

.row-del:hover {
  color: var(--red);
  background: var(--bg-hover);
}

.popover-foot {
  @apply flex items-center justify-between pt-2 mt-1 border-t border-border px-1;
}

.add-btn {
  @apply text-accent bg-transparent border-none cursor-pointer text-xs py-0.5 px-1 rounded-md;
  font: inherit;
}

.add-btn:hover {
  background: var(--accent-soft);
}

.foot-hint {
  @apply text-[11px] text-text-tertiary;
}

.segmented {
  @apply relative flex gap-0.5 p-0.5 rounded-md bg-bg-inset;
}

/* Sliding active pill. Its width is one of four equal segments minus the
   4px padding and 3×2px gaps; translateX steps it one segment (+gap) at a
   time, so the highlight glides to the chosen mode instead of jumping. */
.seg-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 rounded-md bg-bg-panel;
  width: calc((100% - 10px) / 4);
  transition: transform 0.18s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.18), 0 0 0 0.5px var(--border-strong);
}

.seg {
  @apply relative flex-1 text-center text-[11.5px] py-1 rounded-md border-none bg-transparent text-text-secondary cursor-pointer;
  transition: color 0.18s ease;
}

.seg.active {
  @apply font-medium text-text;
}

.body-area {
  @apply w-full min-h-32 rounded-md p-2 text-[11.5px] outline-none select-text;
  font-family: var(--mono);
  background: var(--bg-inset);
  border: 1px solid var(--border);
  color: var(--text);
  resize: vertical;
}

.body-area:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.body-disabled {
  @apply h-16 rounded-md flex items-center justify-center text-[11.5px] text-text-tertiary;
  background: var(--bg-inset);
  border: 1px solid var(--border);
}
</style>
