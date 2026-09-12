<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import VarToken from '../ui/VarToken.vue'
import { PopoverContent } from '../ui/popover'
import { Button, IconButton } from '../ui/button'
import { Checkbox } from '../ui/checkbox'
import { useRequestsStore } from '../../stores/requests'
import { parseTokens, tokenSegments } from '../../lib/vars'

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

// The pill moves one segment (+ the 2px gap) per step, animated by a CSS transition on transform.
const activeAuthIndex = computed(() => AUTH_TYPES.indexOf(store.draft.auth.type))
const authIndicatorStyle = computed(() => ({
  transform: `translateX(calc(${activeAuthIndex.value} * (100% + 2px)))`,
}))

function dismiss() {
  store.setOpenChip(null)
}

// A chip button sits outside the popover's own content, so clicking it — even the one already
// open, to toggle it shut — reaches reka-ui as an outside interaction (both the pointerdown and,
// since a button click also moves focus onto it, the focus that follows) and closes the popover
// on its own; the button's own `click` handler then runs against that now-closed state and
// reopens it. Left alone this reads as "clicking the open chip does nothing" or "reopens instead
// of closing". `interactOutside` covers both paths, so suppressing it for chip clicks hands the
// whole open/close decision to `toggleChip`.
function onInteractOutside(e: Event) {
  const target = (e as CustomEvent<{ originalEvent?: Event }>).detail?.originalEvent?.target
  if (target instanceof HTMLElement && target.closest('.chip')) e.preventDefault()
}

// Same as the URL field: the input stays the editable control and a transparent layer paints tokens above it.
function hasTokens(value: string): boolean {
  return parseTokens(value).length > 0
}

function syncCellScroll(e: Event) {
  const input = e.target as HTMLInputElement
  const display = input.parentElement?.querySelector<HTMLElement>('.row-display')
  if (display) display.scrollLeft = input.scrollLeft
}

// Numbers take --tok-num, everything else --tok-str, mirroring the JSON tree's value highlighting.
function valueClass(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : 'str'
}

</script>

<template>
  <PopoverContent
    class="chip-popover"
    :class="{ auth: props.chip === 'auth', spaced: props.chip === 'auth' || props.chip === 'body' }"
    align="end"
    :side-offset="6"
    @interact-outside="onInteractOutside"
  >
    <div class="popover-head">
      <span class="popover-title">{{ title }}</span>
      <IconButton hint="Закрыть (Esc)" size="sm" @click="dismiss"><Icon name="xmark" :size="13" /></IconButton>
    </div>

    <template v-if="props.chip === 'params' || props.chip === 'headers'">
      <template v-if="props.chip === 'params'">
        <TransitionGroup tag="div" name="row" class="rows">
          <div v-for="(p, i) in store.draft.params" :key="i" class="row" :class="{ off: !p.enabled }">
            <Checkbox :model-value="p.enabled" @update:model-value="store.toggleParam(i)" @click.stop />
            <input :value="p.name" class="row-input mono" placeholder="имя" spellcheck="false" @input="store.updateParam(i, { name: ($event.target as HTMLInputElement).value })" />
            <div class="row-cell">
              <input
                :value="p.value"
                class="row-input mono"
                :class="[valueClass(p.value), { 'row-input-veiled': hasTokens(p.value) }]"
                placeholder="значение"
                spellcheck="false"
                @input="store.updateParam(i, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
                @scroll="syncCellScroll"
              />
              <span
                v-if="hasTokens(p.value)"
                class="row-input row-display mono"
                :class="valueClass(p.value)"
                aria-hidden="true"
              >
                <template v-for="(seg, si) in tokenSegments(p.value)" :key="si">
                  <VarToken v-if="seg.tokenName" :name="seg.tokenName" :offset="seg.start" />
                  <span v-else>{{ seg.text }}</span>
                </template>
              </span>
            </div>
            <IconButton variant="danger" size="sm" hint="Удалить" @click.stop="store.removeParam(i)"><Icon name="xmark" :size="12" /></IconButton>
          </div>
        </TransitionGroup>
        <div class="popover-foot">
          <Button variant="ghost" size="sm" @click="store.addParam()">+ Параметр</Button>
          <span class="foot-hint">Выключенные не уходят в запрос</span>
        </div>
      </template>

      <template v-else>
        <TransitionGroup tag="div" name="row" class="rows">
          <div v-for="(h, i) in store.draft.headers" :key="i" class="row" :class="{ off: !h.enabled }">
            <Checkbox :model-value="h.enabled" @update:model-value="store.toggleHeader(i)" @click.stop />
            <input :value="h.name" class="row-input mono" placeholder="Header" spellcheck="false" @input="store.updateHeader(i, { name: ($event.target as HTMLInputElement).value })" />
            <div class="row-cell">
              <input
                :value="h.value"
                class="row-input mono"
                :class="[valueClass(h.value), { 'row-input-veiled': hasTokens(h.value) }]"
                placeholder="Value"
                spellcheck="false"
                @input="store.updateHeader(i, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
                @scroll="syncCellScroll"
              />
              <span
                v-if="hasTokens(h.value)"
                class="row-input row-display mono"
                :class="valueClass(h.value)"
                aria-hidden="true"
              >
                <template v-for="(seg, si) in tokenSegments(h.value)" :key="si">
                  <VarToken v-if="seg.tokenName" :name="seg.tokenName" :offset="seg.start" />
                  <span v-else>{{ seg.text }}</span>
                </template>
              </span>
            </div>
            <IconButton variant="danger" size="sm" hint="Удалить" @click.stop="store.removeHeader(i)"><Icon name="xmark" :size="12" /></IconButton>
          </div>
        </TransitionGroup>
        <div class="popover-foot">
          <Button variant="ghost" size="sm" @click="store.addHeader()">+ Заголовок</Button>
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
      <div class="hint">Значение можно взять из окружения — переменные подставляются в URL, заголовки и тело.</div>
    </template>

    <template v-else>
      <textarea
        v-if="!isBodyDisabled"
        v-model="store.draft.body"
        class="body-area mono"
        placeholder="{ ... JSON body ... }"
        spellcheck="false"
      ></textarea>
      <div v-else class="body-area body-disabled">{{ store.draft.method }} не отправляет тело</div>
    </template>
  </PopoverContent>
</template>

<style scoped>
@reference "../../style.css";

.chip-popover.spaced .popover-head {
  padding-bottom: 0;
}

.popover-head {
  @apply flex items-center justify-between px-1 pt-0.5 pb-2;
}

.popover-title {
  @apply text-xs font-semibold;
}

.row {
  @apply grid grid-cols-[20px_150px_1fr_22px] gap-1.5 items-center py-[3px] px-1;
}

.row + .row {
  @apply mt-px;
}

.row.off {
  @apply opacity-55;
}

/* Keyed by index, not identity — rows have none (`updateParam`/`updateHeader` replace the object
   on every keystroke) — so removing one above the last animates the last row out and snaps the
   rest into place, rather than animating the row that actually left. */
.row-enter-active,
.row-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

/* No `.row-move` transition on purpose: reka-ui's floating-ui repositions the popover itself a
   frame after mount (its size depends on this very content), and Vue's TransitionGroup reads that
   as its rows having moved — it would animate a translate computed against the popover's own
   pre-position offset, flying the whole row in from wherever the popover started. Left undefined,
   that same correction still runs but resolves in one frame instead of over a visible transition. */

.row-enter-from,
.row-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

.row-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-xs p-0 rounded-sm;
  font-family: var(--mono);
  color: var(--text);
}

.row-input:focus {
  background: var(--bg-inset);
}

.row-cell {
  @apply relative flex min-w-0;
}

.row-cell .row-input.row-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

.row-display {
  @apply absolute inset-0 flex items-center overflow-hidden pointer-events-none;
  white-space: pre;
}

.row-input.str {
  color: var(--tok-str);
}

.row-input.num {
  color: var(--tok-num);
}

.popover-foot {
  @apply flex items-center justify-between pt-2 pb-0.5 mt-1 border-t border-border px-1;
}

.foot-hint {
  @apply text-[11px] text-text-tertiary;
}

.hint {
  color: var(--text-tertiary);
  font-size: 11.5px;
  line-height: 1.45;
  padding: 0 4px 2px;
}

.segmented {
  @apply relative flex gap-0.5 p-0.5 bg-bg-inset;
  border-radius: 7px;
}

.seg-indicator {
  @apply absolute top-0.5 bottom-0.5 left-0.5 bg-bg-panel;
  border-radius: 5px;
  width: calc((100% - 10px) / 4);
  transition: transform 0.18s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.18), 0 0 0 0.5px var(--border-strong);
}

.seg {
  @apply relative flex-1 text-center text-xs py-1 border-none bg-transparent text-text-secondary cursor-pointer;
  border-radius: 5px;
  transition: color 0.18s ease;
}

.seg.active {
  @apply font-medium text-text;
}

.body-area {
  @apply w-full text-xs outline-none select-text;
  font-family: var(--mono);
  height: 132px;
  border-radius: 8px;
  padding: 8px 10px;
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
  color: var(--text-tertiary);
  resize: none;
}
</style>
