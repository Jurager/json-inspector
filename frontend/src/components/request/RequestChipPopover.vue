<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import VarToken from '../ui/VarToken.vue'
import { PopoverContent } from '../ui/popover'
import { Button, IconButton } from '../ui/button'
import { Checkbox } from '../ui/checkbox'
import ScriptsFields from './ScriptsFields.vue'
import BodyFields from './BodyFields.vue'
import AuthFields from './AuthFields.vue'
import DerivedRow from './DerivedRow.vue'
import AuthTypeSwitch from './AuthTypeSwitch.vue'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import type { ChipName, RequestSource } from '../../lib/requestSource'
import { parseTokens, tokenSegments } from '../../lib/vars'
import { useMessages, formatCheckedAt } from '../../i18n'
import { DraftID, RowKind, type Auth } from '../../../bindings/json-inspector/internal/domain'

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

const { schemeOf } = useAuthSchemes()

// The scheme the request is authorized with, as Go describes it: which fields to ask for and how to
// draw them. A card inside a collection may also choose «Наследовать» — the request is one node of a
// tree, and Go offers that scheme only where there is a level above it.
const scheme = computed(() => schemeOf(store.auth.type))

// What the level above answers, said in one line: a choice whose meaning is invisible is a choice
// nobody makes.
const inheritedLabel = computed(() => {
  const auth = store.inheritedAuth
  if (!auth) return t('request.inheritedNothing')
  const above = schemeOf(auth.type)
  const kind = above ? t(above.label) : auth.type
  // The one answer worth repeating is the one that is not a secret: a token is masked in the copy
  // that outlives the send, and the window is not the side that gets to see it.
  return t('request.inherited', { kind })
})

function dismiss() {
  store.setOpenChip(null)
}

// The scheme is a click, so it is written at once. The answers travel with it rather than being
// cleared: each scheme keeps its own, so going to look at Basic and coming back finds the token
// still there — and clicking the scheme already in use is not an answer to anything.
function selectAuth(type: Auth['type']) {
  if (type === store.auth.type) return
  void store.setAuth({ ...store.auth, type })
}

function setField(key: string, value: string) {
  void store.setAuth({ ...store.auth, fields: { ...store.auth.fields, [key]: value } })
}

// What the token block says. A provider that named no expiry never expires to this window's
// knowledge, and saying "получен" is more honest than inventing a time for it.
const tokenState = computed(() => {
  if (!store.token?.held) return t('request.auth.noToken')
  if (!store.token.expiresAt) return t('request.auth.tokenHeld')
  return t('request.auth.tokenUntil', { at: formatCheckedAt(store.token.expiresAt) })
})

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

// The rows the authorization put in the list, kept apart from the ones a person wrote: they are
// drawn after them, and they are edited through the scheme's fields rather than as rows.
function projected(kind: RowKind) {
  return store.projected.filter((row) => row.target === kind)
}

function patchDerived(target: RowKind, name: string, value: string) {
  void store.patchDerived(target, name, value)
}

function removeDerived() {
  void store.removeDerived()
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
      <IconButton :hint="t('common.close')" size="xl" @click="dismiss"><Icon name="xmark" :size="14" /></IconButton>
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
            <IconButton variant="danger" size="xl" :hint="t('common.delete')" @click.stop="remove(RowKind.RowParams, p.id)"><Icon name="trash" :size="13" /></IconButton>
          </div>
        </TransitionGroup>
        <DerivedRow
          v-for="row in projected(RowKind.RowParams)"
          :key="row.name"
          :row="row"
          @patch="patchDerived"
          @remove="removeDerived"
        />
        <div class="popover-foot">
          <Button variant="ghost" size="panel" @click="store.addRow(RowKind.RowParams)">
            <Icon name="plus" :size="15" />
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
            <IconButton variant="danger" size="xl" :hint="t('common.delete')" @click.stop="remove(RowKind.RowHeaders, h.id)"><Icon name="trash" :size="13" /></IconButton>
          </div>
        </TransitionGroup>
        <DerivedRow
          v-for="row in projected(RowKind.RowHeaders)"
          :key="row.name"
          :row="row"
          @patch="patchDerived"
          @remove="removeDerived"
        />
        <div class="popover-foot">
          <Button variant="ghost" size="panel" @click="store.addRow(RowKind.RowHeaders)">
            <Icon name="plus" :size="15" />
            <span>{{ t('request.addHeader') }}</span>
          </Button>
          <span class="foot-hint">{{ t('request.headersFoot') }}</span>
        </div>
      </template>
    </template>

    <template v-else-if="props.chip === 'auth'">
      <AuthTypeSwitch :can-inherit="store.canInherit" :value="store.auth.type" @select="selectAuth" />

      <div class="auth-body">
        <!-- «Нет» and «Наследовать» ask for nothing and say what they mean instead. So does a scheme
             whose working is invisible until the server answers it. -->
        <p v-if="scheme?.note" class="hint">{{ t(scheme.note) }}</p>
        <AuthFields
          v-if="scheme?.fields?.length"
          :scheme="scheme"
          :auth="store.auth"
          @update="setField"
        />
        <!-- The part of an authorization that is not a field: a token somebody else issues, and the
             two things a person can do about it. It is drawn where the scheme says it fetches, so a
             scheme that carries what the user typed gets no block saying there is no token. -->
        <div v-if="scheme?.fetches && store.token" class="token">
          <div class="token-state">
            <Icon name="lock" :size="13" />
            <span>{{ tokenState }}</span>
          </div>
          <div class="token-actions">
            <Button variant="primary" size="sm" @click="store.obtainAuth()">
              {{ t('request.auth.obtain') }}
            </Button>
            <Button variant="outline" size="sm" :disabled="!store.token.held" @click="store.forgetAuth()">
              {{ t('request.auth.forget') }}
            </Button>
          </div>
        </div>
        <p v-if="store.auth.type === 'inherit'" class="hint">{{ inheritedLabel }}</p>
      </div>

      <div class="auth-foot">{{ t('request.auth.foot') }}</div>
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

/* The popover is 14px of padding all round with 4px between what it holds, and its head is a row of
   its own above that rhythm — the design's own numbers for the chip popovers. The head carries no
   padding of its own but the 10px under it, so its title and its close button stand at the popover's
   edge with the rows below. */
.popover-head {
  @apply flex items-center justify-between pb-2.5;
}

.popover-title {
  @apply text-[14px] font-semibold;
}

/* The rows are one block inside the popover, and the 4px between them is the popover's own rhythm
   carried in: the design stacks them on the same gap it stacks the head and the foot on. */
.rows {
  @apply flex flex-col gap-1;
}

.row {
  @apply grid grid-cols-[24px_160px_minmax(0,1fr)_28px] gap-2 items-center h-[38px] px-1.5 rounded-lg;
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
  @apply min-w-0 bg-transparent border-0 outline-none text-[13.5px] p-0 rounded-sm;
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

/* The sentence a scheme says about itself. It stands at the popover's own edge with the fields under
   it, which is where the design puts it. */
.hint {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

/* The part of an authorization that is not a field. It is a block of its own rather than a row of
   the form because it is not an answer the user gives: it is what came back when they asked. */
.token {
  @apply flex flex-col gap-2 p-2.5 rounded-lg bg-bg-inset border border-border;
}

.token-state {
  @apply flex items-center gap-1.5 text-[11.5px] text-text-secondary;
}

.token-state svg {
  @apply flex-none;
}

.token-actions {
  @apply flex gap-1.5;
}

</style>
