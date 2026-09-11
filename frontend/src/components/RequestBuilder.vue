<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { SendRequest, CancelRequest } from '../../wailsjs/go/main/App'
import { useRequestsStore } from '../stores/requests'
import { shortcut } from '../lib/platform'

const store = useRequestsStore()

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

function collectHeaders(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const h of store.draft.headers) {
    const name = h.name.trim()
    if (name && h.enabled) map[name] = h.value
  }
  return map
}

async function send() {
  if (!store.draft.url.trim() || store.loading) return
  store.loading = true
  const requestHeaders = collectHeaders()
  try {
    const res = await SendRequest(store.draft.method, store.draft.url.trim(), requestHeaders, store.draft.body)
    if (res.cancelled) return
    store.add({
      method: store.draft.method,
      url: store.draft.url.trim(),
      requestHeaders,
      requestBody: store.draft.body,
      status: res.status,
      statusText: res.statusText,
      responseHeaders: res.headers,
      responseBody: res.body,
      durationMs: res.durationMs,
      contentType: res.contentType,
      error: res.error,
      source: 'manual',
    })
  } finally {
    store.loading = false
  }
}

async function cancel() {
  await CancelRequest()
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
      <div class="url-field">
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

        <input
          :value="store.draft.url"
          class="url-input mono"
          placeholder="https://api.example.com/articles?include=author"
          spellcheck="false"
          @input="store.setUrl(($event.target as HTMLInputElement).value)"
          @keydown.enter="send"
        />

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
            :disabled="isBodyDisabled"
            :title="isBodyDisabled ? `${store.draft.method} не отправляет тело` : undefined"
            @click="toggleChip('body')"
          >
            Тело
          </button>
        </div>
      </div>

      <button class="btn btn-primary send-btn" :disabled="!store.draft.url.trim()" @click="store.loading ? cancel() : send()">
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

.url-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none px-2 text-[12.5px];
  font-family: var(--mono);
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
