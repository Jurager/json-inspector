<script setup lang="ts">
import Icon from '../ui/Icon.vue'
import { IconButton } from '../ui/button'
import VarToken from '../ui/VarToken.vue'
import { useRequestsStore } from '../../stores/requests'
import { parseTokens, tokenSegments } from '../../lib/vars'
import { RowKind, type CookieRow } from '../../../bindings/json-inspector/internal/domain'

const store = useRequestsStore()

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
</script>

<template>
  <div class="req-cookies">
    <div class="req-cookies-head">
      <div>Имя</div><div>Значение</div><div>Домен</div><div>Истекает</div><div>Флаги</div><div></div>
    </div>
    <div v-for="c in store.cookies" :key="c.id" class="req-cookies-row">
      <input
        :value="c.name"
        class="cell-input mono"
        placeholder="имя"
        spellcheck="false"
        @input="patch(c, { name: ($event.target as HTMLInputElement).value })"
      />
      <div class="cell">
        <input
          :value="c.value"
          class="cell-input mono"
          :class="{ 'cell-input-veiled': hasTokens(c.value) }"
          placeholder="значение"
          spellcheck="false"
          @input="patch(c, { value: ($event.target as HTMLInputElement).value }); syncCellScroll($event)"
          @scroll="syncCellScroll"
        />
        <span v-if="hasTokens(c.value)" class="cell-input row-display mono" aria-hidden="true">
          <template v-for="(seg, si) in tokenSegments(c.value)" :key="si">
            <VarToken v-if="seg.tokenName" :name="seg.tokenName" :offset="seg.start" />
            <span v-else>{{ seg.text }}</span>
          </template>
        </span>
      </div>
      <input
        :value="c.domain"
        class="cell-input"
        placeholder="домен"
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
      <IconButton variant="danger" size="sm" hint="Удалить" @click="store.removeRow(RowKind.RowCookies, c.id ?? '')">
        <Icon name="xmark" :size="12" />
      </IconButton>
    </div>
    <button class="req-cookies-add" @click="store.addRow(RowKind.RowCookies)">
      <Icon name="plus" :size="13" />
      <span>Добавить cookie</span>
    </button>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.req-cookies {
  @apply min-h-full flex flex-col bg-bg-panel;
}

.code {
  @apply rounded-sm py-px px-1.5;
  background: var(--bg-inset);
}

.req-cookies-head,
.req-cookies-row {
  @apply grid items-center px-5 py-3 gap-3;
  grid-template-columns: 150px minmax(0, 1fr) 140px 100px 120px 24px;
}

.req-cookies-head {
  @apply flex-none border-b border-border text-[10px] uppercase tracking-wider text-text-tertiary;
}

.req-cookies-row {
  @apply py-2 flex-none border-b border-border;
  transition: background-color 0.12s ease;
}

.req-cookies-row:hover {
  background: var(--bg-hover);
}

.cell {
  @apply relative flex min-w-0;
}

.cell-input {
  @apply min-w-0 bg-transparent border-0 outline-none text-[12.5px] py-0 px-0 rounded-md;
  color: var(--text);
}

.cell-input:focus {
  background: var(--bg-inset);
}

.cell-input.expires {
  @apply text-xs text-text-secondary;
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
  @apply flex gap-1;
}

.flag-btn {
  @apply text-xs font-semibold py-0.5 px-2 rounded-md border-none cursor-pointer text-text-tertiary;
  background: var(--bg-inset);
  transition: background-color 0.12s ease, color 0.12s ease;
}

.flag-btn.active {
  @apply text-accent;
  background: var(--accent-soft);
}

.req-cookies-add {
  @apply flex-none flex items-center gap-1.5 h-[38px] px-4 border-none bg-transparent text-left cursor-pointer text-text-tertiary;
  font: inherit;
  font-size: 12.5px;
  --wails-draggable: no-drag;
}

.req-cookies-add:hover {
  @apply bg-bg-hover text-text;
}
</style>
