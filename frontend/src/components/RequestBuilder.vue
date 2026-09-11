<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import RequestChipPopover from './RequestChipPopover.vue'
import VarToken from './VarToken.vue'
import { SendRequest, CancelRequest } from '../../wailsjs/go/main/App'
import { useRequestsStore } from '../stores/requests'
import { useEnvironmentsStore } from '../stores/environments'
import { shortcut } from '../lib/platform'
import { segments } from '../lib/vars'

const store = useRequestsStore()
const envStore = useEnvironmentsStore()

const sendShortcut = computed(() => shortcut('↵'))

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

const METHOD_COLORS: Record<string, string> = {
  GET: 'var(--green)',
  POST: 'var(--orange)',
  PUT: 'var(--purple)',
  PATCH: 'var(--purple)',
  DELETE: 'var(--red)',
  HEAD: 'var(--text-tertiary)',
  OPTIONS: 'var(--text-tertiary)',
}

const methodColor = computed(() => METHOD_COLORS[store.draft.method] ?? 'var(--accent)')

const methodBg = computed(() => {
  const c = METHOD_COLORS[store.draft.method] ?? 'var(--accent)'
  return `color-mix(in srgb, ${c} 14%, transparent)`
})

// --- Method dropdown ---
const methodOpen = ref(false)
const methodWrap = ref<HTMLElement | null>(null)

function selectMethod(m: string) {
  store.draft.method = m
  methodOpen.value = false
}

// --- Chip counters ---
const enabledParamsCount = computed(
  () => store.draft.params.filter((p) => p.enabled && p.name.trim()).length
)
const enabledHeadersCount = computed(
  () => store.draft.headers.filter((h) => h.enabled && h.name.trim()).length
)
const hasBody = computed(() => store.draft.body.trim().length > 0)
const isBodyDisabled = computed(() => store.draft.method === 'GET' || store.draft.method === 'HEAD')

function toggleChip(chip: 'params' | 'headers' | 'auth' | 'body') {
  store.setOpenChip(store.openChip === chip ? null : chip)
}

// The values that actually go on the wire: tokens replaced by the active
// environment's values. The draft itself keeps the tokens, so switching
// environments changes what is sent without touching the saved request.
function collectHeaders(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const h of store.draft.headers) {
    const name = envStore.substitute(h.name.trim())
    if (name && h.enabled) map[name] = envStore.substitute(h.value)
  }
  return map
}

// Sending an unresolved `{{name}}` would put the braces on the wire and come
// back as a confusing 404, so the send is blocked until the value exists. Both
// the button and ⌘↵ come through here, so neither can slip past.
const missing = computed(() => store.missingVars)
const sendBlocked = computed(() => missing.value.length > 0)

const blockedHint = computed(() =>
  missing.value.length
    ? `Неизвестные переменные: ${missing.value.join(', ')}`
    : undefined
)

// Creates the missing names in the active environment and drops the user into
// the editor focused on the first one. With no environment chosen there is
// nothing to create into — the menu is the way out of that state.
function createMissing() {
  const envId = envStore.activeId
  if (envId === null) return
  for (const name of missing.value) envStore.addVar(envId, { name })
  envStore.openSheet({ envId, varName: missing.value[0] ?? '' })
}

async function send() {
  if (!store.draft.url.trim() || store.loading || sendBlocked.value) return
  store.loading = true
  const requestHeaders = collectHeaders()
  const url = envStore.substitute(store.draft.url.trim())
  const body = envStore.substitute(store.draft.body)
  try {
    const res = await SendRequest(store.draft.method, url, requestHeaders, body)
    if (res.cancelled) return
    // The record keeps the resolved request, not the template: the "Запрос"
    // tab is there to show what really left the machine.
    store.add({
      method: store.draft.method,
      url,
      requestHeaders,
      requestBody: body,
      status: res.status,
      statusText: res.statusText,
      responseHeaders: res.headers,
      responseBody: res.body,
      durationMs: res.durationMs,
      contentType: res.contentType,
      error: res.error,
      dnsMs: res.dnsMs,
      connectMs: res.connectMs,
      tlsMs: res.tlsMs,
      waitMs: res.waitMs,
      downloadMs: res.downloadMs,
      source: 'manual',
    })
  } finally {
    store.loading = false
  }
}

async function cancel() {
  await CancelRequest()
}

// Focus the URL field when "Открыть в «Запросе»" asks for it — the field only
// exists after this component remounts, hence the nextTick.
const urlInputRef = ref<HTMLInputElement | null>(null)

watch(
  () => store.focusUrlTick,
  () => {
    nextTick(() => urlInputRef.value?.focus())
  }
)

// --- Token highlighting in the URL field ---
//
// The input stays the real, editable control; highlights are painted by a
// separate layer above it. The input's own glyphs are hidden (not removed) and
// the layer is transparent to the mouse, so the caret, selection, drag-select
// and the IME path all keep working natively — only tokens opt back into mouse
// events, for their hover tooltip.
const urlDisplayRef = ref<HTMLElement | null>(null)

const urlSegments = computed(() => segments(store.draft.url))
const showUrlDisplay = computed(() => urlSegments.value.length > 0)

// A long URL scrolls while it's typed; the layer above has to follow, or the
// two texts drift apart.
function syncUrlScroll() {
  const input = urlInputRef.value
  const display = urlDisplayRef.value
  if (input && display) display.scrollLeft = input.scrollLeft
}

// Close the method dropdown and the chip popover on an outside click. The
// popover is a descendant of `.request-bar` (absolutely positioned below it),
// so a click inside it still counts as "inside" and keeps it open.
const requestBarEl = ref<HTMLElement | null>(null)

function onDocClick(e: MouseEvent) {
  const t = e.target as Node
  if (methodWrap.value && !methodWrap.value.contains(t)) methodOpen.value = false
  if (requestBarEl.value && !requestBarEl.value.contains(t)) store.setOpenChip(null)
}

function onWindowKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    send()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onWindowKeydown)
  document.addEventListener('click', onDocClick)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKeydown)
  document.removeEventListener('click', onDocClick)
})
</script>

<template>
  <div class="builder">
    <div ref="requestBarEl" class="request-bar">
      <div class="url-field" :class="{ 'url-field-invalid': sendBlocked }">
        <div ref="methodWrap" class="method-wrap">
          <button class="method-btn" :style="{ color: methodColor, background: methodBg }" @click="methodOpen = !methodOpen">
            <span>{{ store.draft.method }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
          <div v-if="methodOpen" class="menu method-menu">
            <button
              v-for="m in METHODS"
              :key="m"
              class="menu-item"
              :style="{ color: METHOD_COLORS[m] ?? 'var(--accent)' }"
              @click="selectMethod(m)"
            >
              {{ m }}
            </button>
          </div>
        </div>

        <div class="url-text">
          <input
            ref="urlInputRef"
            :value="store.draft.url"
            class="url-input mono"
            :class="{ 'url-input-veiled': showUrlDisplay }"
            placeholder="https://api.example.com/articles?include=author"
            spellcheck="false"
            @input="store.setUrl(($event.target as HTMLInputElement).value); syncUrlScroll()"
            @keydown.enter="send"
            @scroll="syncUrlScroll"
          />
          <!-- Decorative: the input above holds the real value and stays the
               only editable control. -->
          <div v-if="showUrlDisplay" ref="urlDisplayRef" class="url-display mono" aria-hidden="true">
            <template v-for="(seg, i) in urlSegments" :key="i">
              <VarToken v-if="seg.token" :name="seg.token" :offset="seg.start" />
              <span v-else>{{ seg.text }}</span>
            </template>
          </div>
        </div>

        <div class="chips">
          <button class="chip" :class="{ active: store.openChip === 'params' }" @click="toggleChip('params')">
            Параметры <span v-if="enabledParamsCount" class="chip-count">{{ enabledParamsCount }}</span>
          </button>
          <button class="chip" :class="{ active: store.openChip === 'headers' }" @click="toggleChip('headers')">
            Заголовки <span v-if="enabledHeadersCount" class="chip-count">{{ enabledHeadersCount }}</span>
          </button>
          <button class="chip" :class="{ active: store.openChip === 'auth' }" @click="toggleChip('auth')">Auth</button>
          <button
            class="chip chip-body"
            :class="{ 'has-body': hasBody, active: store.openChip === 'body' }"
            :title="isBodyDisabled ? `${store.draft.method} не отправляет тело` : undefined"
            @click="toggleChip('body')"
          >
            Тело
          </button>
        </div>
      </div>

      <button
        class="btn btn-primary send-btn"
        :disabled="!store.draft.url.trim() || sendBlocked"
        :title="blockedHint"
        @click="store.loading ? cancel() : send()"
      >
        <template v-if="store.loading">
          <Icon name="xmark" :size="14" />
          <span>Отмена</span>
        </template>
        <template v-else>
          <span>Отправить</span>
          <kbd class="send-hint">{{ sendShortcut }}</kbd>
        </template>
      </button>

      <RequestChipPopover v-if="store.openChip" :chip="store.openChip" />
    </div>

    <!-- The one thing allowed to add height to the command line: an unresolved
         variable blocks the request outright, and the fix is one click away. -->
    <div v-if="sendBlocked" class="missing-row">
      <span class="missing-text">
        <template v-if="envStore.activeId === null">
          Окружение не выбрано — переменные
          <span class="missing-name mono" v-for="n in missing" :key="n">{{ n }}</span>
          не подставляются. Отправка заблокирована.
        </template>
        <template v-else>
          В окружении <b>{{ envStore.active?.name }}</b> нет
          {{ missing.length === 1 ? 'переменной' : 'переменных' }}:
          <span class="missing-name mono" v-for="n in missing" :key="n">{{ n }}</span>
          Отправка заблокирована.
        </template>
      </span>
      <button
        class="btn btn-primary missing-create"
        :disabled="envStore.activeId === null"
        :title="envStore.activeId === null ? 'Сначала выберите окружение в шапке' : undefined"
        @click="createMissing"
      >
        {{ missing.length === 1 ? 'Создать' : 'Создать все' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.builder {
  @apply flex-none border-b border-border bg-bg-panel;
}

/* The command line is exactly one 56px row — nothing inside it expands
   downward; settings open as a popover layered over the response below. */
.request-bar {
  @apply relative flex items-center gap-2 h-14 py-3 px-4;
}

.url-field {
  @apply flex-1 flex items-stretch rounded-lg border border-border bg-bg-inset h-8;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.url-field:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

/* An unresolved variable is a hard stop, so the field keeps its red edge even
   while focused — the accent ring would read as "all good". */
.url-field.url-field-invalid,
.url-field.url-field-invalid:focus-within {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
}

.url-field.url-field-invalid:focus-within {
  box-shadow: 0 0 0 3px var(--red-soft);
}

.missing-row {
  @apply flex-none flex items-center gap-3 mx-4 mb-3 py-2 px-2.5 rounded-lg text-[12.5px];
  background: color-mix(in srgb, var(--red) 7%, transparent);
}

.missing-text {
  @apply flex-1 min-w-0;
}

.missing-name {
  @apply text-red mx-1;
}

.missing-create {
  @apply flex-none text-xs;
}

.method-wrap {
  @apply relative flex items-center flex-none p-1;
}

.method-btn {
  @apply flex items-center gap-1 rounded-md border-0 font-semibold text-[11px] cursor-pointer outline-none h-[22px];
  padding: 0 7px;
  font-family: var(--mono);
}

.method-menu {
  @apply absolute top-full left-0 mt-1;
}

.url-text {
  @apply relative flex-1 min-w-0 flex items-stretch;
}

.url-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none px-2 text-[12.5px];
  font-family: var(--mono);
  color: var(--text);
}

/* The display layer sits exactly on top of the input, so the input's own glyphs
   would show through doubled — they are hidden, not removed, and the caret is
   given its colour back explicitly (it follows `color`, so it would otherwise
   vanish with them). */
.url-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

/* Transparent to the mouse: every click, drag and caret placement goes to the
   input underneath. Tokens opt back in individually for their tooltip. */
.url-display {
  @apply absolute inset-0 flex items-center overflow-hidden px-2 text-[12.5px] pointer-events-none;
  font-family: var(--mono);
  white-space: pre;
  color: var(--text);
}

.url-input:focus,
.url-input:focus-visible {
  border: 0;
  box-shadow: none;
}

.chips {
  @apply flex-none flex items-center gap-1 pr-1.5;
}

.chip {
  @apply flex items-center gap-1 h-[22px] px-2 rounded-md text-[11px] text-text-secondary border border-border bg-bg-panel cursor-pointer;
  transition: border-color 0.12s ease, color 0.12s ease;
  --wails-draggable: no-drag;
}

.chip:hover {
  border-color: var(--border-strong);
  color: var(--text);
}

.chip.active {
  border-color: var(--accent);
  color: var(--text);
}

.chip-count {
  @apply font-semibold text-accent;
  font-family: var(--mono);
}

.chip-body {
  border-style: dashed;
  border-color: color-mix(in srgb, var(--text-tertiary) 40%, transparent);
  color: var(--text-tertiary);
  background: transparent;
}

.chip-body.has-body {
  border-style: solid;
  border-color: var(--border);
  color: var(--text);
  background: var(--bg-panel);
}

.chip-body:disabled {
  @apply opacity-50 cursor-default;
}

.send-btn {
  @apply flex-none h-8 inline-flex items-center justify-center gap-1.5 px-3.5 rounded-lg;
  border-radius: 8px;
}

.send-hint {
  @apply text-[10px] font-medium leading-normal py-0 px-1.5 ml-1 rounded-sm text-white;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.22);
}
</style>
