<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import RequestChipPopover from './RequestChipPopover.vue'
import { Button } from '../ui/button'
import { Popover, PopoverAnchor } from '../ui/popover'
import VarToken from './VarToken.vue'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'
import { App as Backend } from '../../../bindings/json-inspector'
import { useRequestsStore } from '../../stores/requests'
import { useEnvironmentsStore } from '../../stores/environments'
import { shortcut } from '../../lib/platform'
import { segments } from '../../lib/vars'
import { normalizeHeaders } from '../../lib/http'

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

function selectMethod(m: string) {
  store.draft.method = m
}

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

function collectHeaders(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const h of store.draft.headers) {
    const name = envStore.substitute(h.name.trim())
    if (name && h.enabled) map[name] = envStore.substitute(h.value)
  }
  return map
}

// The masked shape the preview and exports read - a credential must never land in the record.
function maskedHeaders(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const h of store.draft.headers) {
    const name = envStore.masked(h.name.trim())
    if (name && h.enabled) map[name] = envStore.masked(h.value)
  }
  return map
}

// An unresolved `{{name}}` would go on the wire as braces and come back a confusing 404, so send is blocked.
const missing = computed(() => store.missingVars)
const sendBlocked = computed(() => missing.value.length > 0)

const blockedHint = computed(() =>
  missing.value.length
    ? `Неизвестные переменные: ${missing.value.join(', ')}`
    : undefined
)

// Opens the editor focused on the first new name; with no environment chosen there is nothing to create into.
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
  // What the record keeps: resolved like the real request, but a secret stays masked.
  const recordUrl = envStore.masked(store.draft.url.trim())
  const recordBody = envStore.masked(store.draft.body)
  const recordHeaders = maskedHeaders()
  try {
    const res = await Backend.SendRequest(store.draft.method, url, requestHeaders, body)
    if (!res || res.cancelled) return
    store.add({
      method: store.draft.method,
      url: recordUrl,
      requestHeaders: recordHeaders,
      requestBody: recordBody,
      status: res.status,
      statusText: res.statusText,
      responseHeaders: normalizeHeaders(res.headers),
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
  await Backend.CancelRequest()
}

// The field only exists after this component remounts, hence the nextTick.
const urlInputRef = ref<HTMLInputElement | null>(null)

watch(
  () => store.focusUrlTick,
  () => {
    nextTick(() => urlInputRef.value?.focus())
  }
)

const urlDisplayRef = ref<HTMLElement | null>(null)

const urlSegments = computed(() => segments(store.draft.url))
const showUrlDisplay = computed(() => urlSegments.value.length > 0)

// The layer above has to follow the input's scroll, or the two texts drift apart.
function syncUrlScroll() {
  const input = urlInputRef.value
  const display = urlDisplayRef.value
  if (input && display) display.scrollLeft = input.scrollLeft
}

function onWindowKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    send()
  }
}

onMounted(() => window.addEventListener('keydown', onWindowKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onWindowKeydown))
</script>

<template>
  <div class="builder">
    <div ref="requestBarEl" class="request-bar">
      <div class="url-field" :class="{ 'url-field-invalid': sendBlocked }">
        <div class="method-wrap">
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="method-btn" :style="{ color: methodColor, background: methodBg }">
                <span>{{ store.draft.method }}</span>
                <Icon name="chevron-down" :size="10" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" :side-offset="4">
              <DropdownMenuItem
                v-for="m in METHODS"
                :key="m"
                :style="{ color: METHOD_COLORS[m] ?? 'var(--accent)' }"
                @select="selectMethod(m)"
              >
                {{ m }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
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
          <!-- Decorative: the input above holds the real value and is the only editable control.
          -->
          <div v-if="showUrlDisplay" ref="urlDisplayRef" class="url-display mono" aria-hidden="true">
            <template v-for="(seg, i) in urlSegments" :key="i">
              <VarToken v-if="seg.token" :name="seg.token" :offset="seg.start" />
              <span v-else>{{ seg.text }}</span>
            </template>
          </div>
        </div>

        <Popover :open="store.openChip !== null" @update:open="(v) => !v && store.setOpenChip(null)">
          <PopoverAnchor class="chips">
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
          </PopoverAnchor>
          <RequestChipPopover v-if="store.openChip" :chip="store.openChip" />
        </Popover>
      </div>

      <Button
        variant="primary"
        size="lg"
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
      </Button>

    </div>

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
      <Button
        variant="primary"
        :disabled="envStore.activeId === null"
        :title="envStore.activeId === null ? 'Сначала выберите окружение в шапке' : undefined"
        @click="createMissing"
      >
        {{ missing.length === 1 ? 'Создать' : 'Создать все' }}
      </Button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.builder {
  @apply flex-none border-b border-border bg-bg-panel;
}

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

.method-wrap {
  @apply relative flex items-center flex-none p-1;
}

.method-btn {
  @apply flex items-center gap-1 rounded-md border-0 font-semibold text-[11px] cursor-pointer outline-none h-[22px];
  padding: 0 7px;
  font-family: var(--mono);
}

.url-text {
  @apply relative flex-1 min-w-0 flex items-stretch;
}

.url-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none px-2 text-[12.5px];
  font-family: var(--mono);
  color: var(--text);
}

.url-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

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

.send-hint {
  @apply text-[10px] font-medium leading-normal py-0 px-1.5 ml-1 rounded-sm text-white;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.22);
}
</style>
