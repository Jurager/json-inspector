<script setup lang="ts">
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import VarToken from '../ui/VarToken.vue'
import { useRequestsStore } from '../../stores/requests'
import { Translation as I18nT } from 'vue-i18n'
import { parseTokens, tokenSegments } from '../../lib/vars'
import { RowKind, type CookieRow } from '../../../bindings/json-inspector/internal/domain'
import { useMessages } from '../../i18n'

const store = useRequestsStore()
const { t } = useMessages()

// Only the value gets the token overlay (same trick as Params/Headers) — a cookie's name is
// realistically always a literal, and the domain/expires columns are informational, not sent.
function hasTokens(value: string): boolean {
  return parseTokens(value).length > 0
}

// The jar is Go's: a row is addressed by id, and an edit goes over as a patch. That is what makes a
// click land on the row it was aimed at even if another one left first.
function patch(row: CookieRow, change: { name?: string; value?: string; domain?: string; expires?: string }) {
  void store.patchRow(RowKind.RowCookies, row.id ?? '', change)
}

function toggleFlag(row: CookieRow, flag: 'secure' | 'httpOnly') {
  void store.patchRow(RowKind.RowCookies, row.id ?? '', { [flag]: !row[flag] })
}

function syncCellScroll(e: Event) {
  const input = e.target as HTMLInputElement
  const display = input.parentElement?.querySelector<HTMLElement>('.row-display')
  if (display) display.scrollLeft = input.scrollLeft
}

// A cookie's value is coloured the way every other value the window shows is coloured: numbers take the
// number token, everything else the string one. Its name is left plain — a cookie name is a literal.
function valueClass(v: string): string {
  return /^-?\d+(\.\d+)?$/.test(v.trim()) ? 'num' : 'str'
}
</script>

<template>
  <div class="req-cookies">
    <!-- What the table is a table of. The word the cookies travel under is drawn as the chip the
         window draws every other literal in. -->
    <I18nT keypath="response.cookies.intro" tag="p" class="intro">
      <template #word><span class="code">{{ t('response.cookies.header') }}</span></template>
    </I18nT>

    <div class="card">
      <div class="req-cookies-head">
        <div>{{ t('response.cookies.name') }}</div><div>{{ t('response.cookies.value') }}</div><div>{{ t('response.cookies.domain') }}</div><div>{{ t('response.cookies.expires') }}</div><div>{{ t('response.cookies.flags') }}</div><div></div>
      </div>
      <div v-for="c in store.cookies" :key="c.id" class="req-cookies-row">
        <input
          :value="c.name"
          class="cell-input mono"
          :placeholder="t('request.placeholderName')"
          spellcheck="false"
          @input="patch(c, { name: ($event.target as HTMLInputElement).value })"
        />
        <div class="cell">
          <input
            :value="c.value"
            class="cell-input mono"
            :class="[valueClass(c.value), { 'cell-input-veiled': hasTokens(c.value) }]"
            :placeholder="t('request.placeholderValue')"
            spellcheck="false"
            @input="patch(c, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
            @scroll="syncCellScroll"
          />
          <span v-if="hasTokens(c.value)" class="cell-input row-display mono" aria-hidden="true">
            <template v-for="(seg, si) in tokenSegments(c.value)" :key="si">
              <VarToken
                v-if="seg.tokenName"
                :name="seg.tokenName"
                :text="seg.text"
                :offset="seg.start"
              />
              <span v-else>{{ seg.text }}</span>
            </template>
          </span>
        </div>
        <input
          :value="c.domain"
          class="cell-input domain"
          :placeholder="t('response.cookies.domain')"
          spellcheck="false"
          @input="patch(c, { domain: ($event.target as HTMLInputElement).value })"
        />
        <input
          :value="c.expires"
          class="cell-input expires"
          placeholder="Session"
          spellcheck="false"
          @input="patch(c, { expires: ($event.target as HTMLInputElement).value })"
        />
        <div class="req-cookies-flags">
          <button
            class="flag-btn"
            :class="{ active: c.secure }"
            @click="toggleFlag(c, 'secure')"
          >
            Secure
          </button>
          <button
            class="flag-btn"
            :class="{ active: c.httpOnly }"
            @click="toggleFlag(c, 'httpOnly')"
          >
            HttpOnly
          </button>
        </div>
        <IconButton variant="danger" size="xl" :hint="t('common.delete')" @click="store.removeRow(RowKind.RowCookies, c.id ?? '')">
          <Icon name="trash" :size="14" :stroke-width="1.8" />
        </IconButton>
      </div>
      <button class="req-cookies-add" @click="store.addRow(RowKind.RowCookies)">
        <Icon name="plus" :size="15" />
        <span>{{ t('response.cookies.add') }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.req-cookies {
  @apply min-h-full flex flex-col gap-3 bg-bg-panel;
  padding: 18px 20px;
}

.intro {
  @apply text-[13px] text-text-secondary;
}

/* The word the jar travels under, in the fill the window gives every literal it quotes. */
.code {
  @apply rounded-[5px] py-0.5 px-1.5 text-[12.5px];
  font-family: var(--mono);
  background: var(--bg-hover);
}

/* The table is a card of its own on the tab's surface, and it clips the header's fill and the rows'
   hover to its own corners. */
.card {
  @apply flex flex-col overflow-hidden border border-border rounded-xl;
}

/* The design's five columns, and a sixth for the one control a jar has that a printed table does not:
   every row is editable and every row can go. */
.req-cookies-head,
.req-cookies-row {
  @apply grid items-center;
  grid-template-columns: 170px minmax(0, 1fr) 160px 130px 170px 28px;
}

.req-cookies-head {
  @apply flex-none bg-bg-inset border-b border-border text-[11px] font-semibold uppercase tracking-[0.06em] text-text-tertiary;
}

.req-cookies-head > div {
  padding: 10px 16px;
}

.req-cookies-row {
  @apply flex-none min-h-12 border-b border-border;
  transition: background-color 0.12s ease;
}

.req-cookies-row:hover {
  background: var(--bg-hover);
}

.cell {
  @apply relative flex min-w-0;
  padding: 0 16px;
}

.cell-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-[13.5px] py-0 rounded-md;
  color: var(--text);
  padding: 0 16px;
}

.cell .cell-input {
  padding: 0;
}

.cell-input:focus {
  background: var(--bg-inset);
}

/* The two columns the request never sends: they are what the jar says about the cookie, not the
   cookie, so they are drawn as text rather than as mono fare. */
.cell-input.domain,
.cell-input.expires {
  @apply text-[13px] text-text-secondary;
}

/* Written before the veiled rule below, which paints the input's own text out: on a tie the later one
   has to be the transparent one, or a value holding a variable would show twice. */
.cell-input.str {
  color: var(--tok-str);
}

.cell-input.num {
  color: var(--tok-num);
}

.cell-input.cell-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

.row-display {
  @apply absolute inset-0 flex items-center overflow-hidden pointer-events-none;
  white-space: pre;
}

.req-cookies-flags {
  @apply flex gap-1.5;
  padding: 0 16px;
}

/* A flag is a state, not a control with a colour of its own: on is the window's "yes", off is the
   chip the row would otherwise be written on. */
.flag-btn {
  @apply text-[11.5px] font-semibold py-[3px] px-2 rounded-md border-none cursor-pointer text-text-tertiary;
  background: var(--bg-hover);
  transition: background-color 0.12s ease, color 0.12s ease;
}

.flag-btn.active {
  @apply bg-green-soft;
  color: var(--green-text);
}

/* The last row of the card rather than a button under it: a jar is a list you add to, and the row that
   adds is the list's own end. */
.req-cookies-add {
  @apply flex-none flex items-center gap-2.5 h-[46px] px-4 border-none bg-transparent text-left cursor-pointer text-text-tertiary;
  font: inherit;
  font-size: 13.5px;
  --wails-draggable: no-drag;
}

.req-cookies-add:hover {
  @apply bg-bg-hover text-text;
}
</style>
