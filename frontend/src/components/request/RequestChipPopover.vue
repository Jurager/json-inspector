<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import VarToken from '../ui/VarToken.vue'
import { PopoverContent } from '../ui/popover'
import { Button, IconButton } from '../ui/button'
import { Checkbox } from '../ui/checkbox'
import ScriptsFields from './ScriptsFields.vue'
import BodyFields from './BodyFields.vue'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import type { ChipName, RequestSource } from '../../lib/requestSource'
import { parseTokens, tokenSegments } from '../../lib/vars'
import { useMessages } from '../../i18n'
import { AuthType, DraftID, RowKind, type Auth } from '../../../bindings/json-inspector/internal/domain'

const props = defineProps<{ chip: ChipName; source?: RequestSource }>()

const { t } = useMessages()

const requests = useRequestsStore()
const collections = useCollectionsStore()
const store: RequestSource = props.source ?? requests

const title = computed(() => t(`request.chips.${props.chip}`))

// Where the code sits in the order, in one line. On the command line there is no collection above the
// request, so the sentence the design gives the card would be saying something untrue there.
const scriptsNote = computed(() =>
  store.scriptsLevel === DraftID.DraftCommandLine
    ? t('request.scriptsNoteCommandLine')
    : t('request.scriptsNoteCollection')
)

// The choices and their names come from Go, so the chip and the draft cannot disagree about what a
// mode is called. A card inside a collection gets «Наследовать» in place of «Нет»: the request is one
// node of a tree, and the design puts the collection's own answer above it.
const AUTH_TYPES = computed<Auth['type'][]>(() =>
  store.canInherit
    ? [AuthType.AuthInherit, AuthType.AuthBearer, AuthType.AuthBasic, AuthType.AuthOAuth2]
    : [AuthType.AuthNone, AuthType.AuthBearer, AuthType.AuthBasic, AuthType.AuthOAuth2]
)
// Rebuilt when the language moves, which is what a static map cannot do — the words are in the
// catalogue and a language is chosen while the window is open.
const AUTH_LABELS = computed<Record<string, string>>(() => ({
  none: t('request.auth.none'),
  inherit: t('request.auth.inherit'),
  bearer: t('request.auth.bearer'),
  basic: t('request.auth.basic'),
  oauth2: t('request.auth.oauth2'),
}))

// What the level above answers, said in one line where the token would be: a choice whose meaning is
// invisible is a choice nobody makes.
const inheritedLabel = computed(() => {
  const auth = store.inheritedAuth
  if (!auth) return t('request.inheritedNothing')
  const kind = AUTH_LABELS.value[auth.type] ?? auth.type
  return auth.token
    ? t('request.inheritedWithToken', { kind, token: auth.token })
    : t('request.inherited', { kind })
})

// The pill moves one segment (+ the 2px gap) per step, animated by a CSS transition on transform.
const activeAuthIndex = computed(() => AUTH_TYPES.value.indexOf(store.auth.type))
const authIndicatorStyle = computed(() => ({
  transform: `translateX(calc(${activeAuthIndex.value} * (100% + 2px)))`,
}))

function dismiss() {
  store.setOpenChip(null)
}

function selectAuth(type: Auth['type']) {
  void store.setAuth({ ...store.auth, type })
}

function setToken(token: string) {
  void store.setAuth({ ...store.auth, token })
}

// A row edit goes straight over: the rows are Go's, and the answer is what the popover draws.
function patch(kind: RowKind, id: string, patch: { name?: string; value?: string }) {
  void store.patchRow(kind, id, patch)
}

function toggle(kind: RowKind, id: string, enabled: boolean) {
  void store.toggleRow(kind, id, enabled)
}

function remove(kind: RowKind, id: string) {
  void store.removeRow(kind, id)
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
    :class="{
      auth: props.chip === 'auth',
      spaced: props.chip === 'auth' || props.chip === 'body' || props.chip === 'scripts',
      wide: props.chip === 'body',
    }"
    align="end"
    :side-offset="6"
    @interact-outside="onInteractOutside"
  >
    <div class="popover-head">
      <span class="popover-title">{{ title }}</span>
      <IconButton :hint="t('common.close')" size="sm" @click="dismiss"><Icon name="xmark" :size="13" /></IconButton>
    </div>

    <template v-if="props.chip === 'params' || props.chip === 'headers'">
      <template v-if="props.chip === 'params'">
        <TransitionGroup tag="div" name="row" class="rows">
          <div v-for="p in store.params" :key="p.id" class="row" :class="{ off: !p.enabled }">
            <Checkbox :model-value="p.enabled" @update:model-value="toggle(RowKind.RowParams, p.id, $event)" @click.stop />
            <input :value="p.name" class="row-input mono" :placeholder="t('request.placeholderName')" spellcheck="false" @input="patch(RowKind.RowParams, p.id, { name: ($event.target as HTMLInputElement).value })" />
            <div class="row-cell">
              <input
                :value="p.value"
                class="row-input mono"
                :class="[valueClass(p.value), { 'row-input-veiled': hasTokens(p.value) }]"
                :placeholder="t('request.placeholderValue')"
                spellcheck="false"
                @input="patch(RowKind.RowParams, p.id, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
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
            <IconButton variant="danger" size="sm" :hint="t('common.delete')" @click.stop="remove(RowKind.RowParams, p.id)"><Icon name="trash" :size="13" /></IconButton>
          </div>
        </TransitionGroup>
        <div class="popover-foot">
          <Button variant="ghost" size="sm" @click="store.addRow(RowKind.RowParams)">
            <Icon name="plus" :size="16" />
            <span>{{ t('request.addParameter') }}</span>
          </Button>
          <span class="foot-hint">{{ t('request.paramsFoot') }}</span>
        </div>
      </template>

      <template v-else>
        <TransitionGroup tag="div" name="row" class="rows">
          <div v-for="h in store.headers" :key="h.id" class="row" :class="{ off: !h.enabled }">
            <Checkbox :model-value="h.enabled" @update:model-value="toggle(RowKind.RowHeaders, h.id, $event)" @click.stop />
            <input :value="h.name" class="row-input mono" placeholder="Header" spellcheck="false" @input="patch(RowKind.RowHeaders, h.id, { name: ($event.target as HTMLInputElement).value })" />
            <div class="row-cell">
              <input
                :value="h.value"
                class="row-input mono"
                :class="[valueClass(h.value), { 'row-input-veiled': hasTokens(h.value) }]"
                placeholder="Value"
                spellcheck="false"
                @input="patch(RowKind.RowHeaders, h.id, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
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
            <IconButton variant="danger" size="sm" :hint="t('common.delete')" @click.stop="remove(RowKind.RowHeaders, h.id)"><Icon name="trash" :size="13" /></IconButton>
          </div>
        </TransitionGroup>
        <div class="popover-foot">
          <Button variant="ghost" size="sm" @click="store.addRow(RowKind.RowHeaders)">
            <Icon name="plus" :size="16" />
            <span>{{ t('request.addHeader') }}</span>
          </Button>
          <span class="foot-hint">{{ t('request.headersFoot') }}</span>
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
          :class="{ active: store.auth.type === t }"
          @click="selectAuth(t)"
        >
          {{ AUTH_LABELS[t] }}
        </button>
      </div>
      <input
        v-if="store.auth.type !== 'none' && store.auth.type !== 'inherit'"
        :value="store.auth.token"
        class="row-input mono"
        :placeholder="t('request.token')"
        spellcheck="false"
        @input="setToken(($event.target as HTMLInputElement).value)"
      />
      <div v-else-if="store.auth.type === 'inherit'" class="hint">{{ inheritedLabel }}</div>
      <div class="hint">{{ t('request.tokenFromEnvironment') }}</div>
    </template>

    <template v-else-if="props.chip === 'scripts'">
      <ScriptsFields :source="store" :note="scriptsNote" compact />
    </template>

    <template v-else>
      <BodyFields :source="store" />
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

</style>
