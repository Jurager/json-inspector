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

const method = ref('GET')
const url = ref('')
const body = ref('')

const methodColor = computed(() => METHOD_COLORS[method.value] ?? 'var(--accent)')

const methodBg = computed(() => {
  const c = METHOD_COLORS[method.value] ?? 'var(--accent)'
  return `color-mix(in srgb, ${c} 14%, transparent)`
})

const methodOpen = ref(false)
const methodWrap = ref<HTMLElement | null>(null)

function selectMethod(m: string) {
  method.value = m
  methodOpen.value = false
}

function onDocClick(e: MouseEvent) {
  if (methodWrap.value && !methodWrap.value.contains(e.target as Node)) {
    methodOpen.value = false
  }
}

type BuilderTab = 'params' | 'headers' | 'body'
const activeTab = ref<BuilderTab>('headers')

interface KeyValue {
  name: string
  value: string
}

const params = ref<KeyValue[]>([])
const headers = ref<KeyValue[]>([{ name: 'Accept', value: 'application/vnd.api+json' }])

function addParam() {
  params.value.push({ name: '', value: '' })
}

function removeParam(i: number) {
  params.value.splice(i, 1)
}

function addHeader() {
  headers.value.push({ name: '', value: '' })
}

function removeHeader(i: number) {
  headers.value.splice(i, 1)
}

function collectHeaders(): Record<string, string> {
  const map: Record<string, string> = {}
  for (const h of headers.value) {
    const name = h.name.trim()
    if (name) map[name] = h.value
  }
  return map
}

function buildUrl(): string {
  const base = url.value.trim()
  const active = params.value.filter((p) => p.name.trim())
  if (active.length === 0) return base
  const qs = active
    .map((p) => `${encodeURIComponent(p.name.trim())}=${encodeURIComponent(p.value)}`)
    .join('&')
  return base.includes('?') ? `${base}&${qs}` : `${base}?${qs}`
}

async function send() {
  if (!url.value.trim() || store.loading) return
  store.loading = true
  const requestHeaders = collectHeaders()
  const finalUrl = buildUrl()
  try {
    const res = await SendRequest(method.value, finalUrl, requestHeaders, body.value)
    if (res.cancelled) return
    store.add({
      method: method.value,
      url: finalUrl,
      requestHeaders,
      requestBody: body.value,
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
    <div class="request-bar">
      <div class="url-field">
        <div ref="methodWrap" class="method-wrap">
          <button class="method-btn" :style="{ color: methodColor, background: methodBg }" @click="methodOpen = !methodOpen">
            <span>{{ method }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
          <div v-if="methodOpen" class="method-menu">
            <button
              v-for="m in METHODS"
              :key="m"
              class="method-item"
              :style="{ color: METHOD_COLORS[m] ?? 'var(--accent)' }"
              @click="selectMethod(m)"
            >
              {{ m }}
            </button>
          </div>
        </div>
        <input
          v-model="url"
          class="url-input mono"
          placeholder="https://api.example.com/articles?include=author"
          spellcheck="false"
          @keydown.enter="send"
        />
      </div>
      <button class="btn btn-primary send-btn" :disabled="!url.trim()" @click="store.loading ? cancel() : send()">
        <template v-if="store.loading">
          <Icon name="xmark" :size="14" />
          <span>Отмена</span>
        </template>
        <template v-else>
          <span>Отправить</span>
          <kbd class="send-hint">{{ sendShortcut }}</kbd>
        </template>
      </button>
    </div>

    <div class="tabs">
      <button class="tab" :class="{ active: activeTab === 'params' }" @click="activeTab = 'params'">Параметры</button>
      <button class="tab" :class="{ active: activeTab === 'headers' }" @click="activeTab = 'headers'">
        Заголовки ({{ headers.filter((h) => h.name.trim()).length }})
      </button>
      <button class="tab" :class="{ active: activeTab === 'body' }" @click="activeTab = 'body'">Тело</button>
    </div>

    <div v-if="activeTab === 'params'" class="builder-panel">
      <div v-for="(p, i) in params" :key="i" class="header-row">
        <input v-model="p.name" class="input header-name mono" placeholder="Ключ" spellcheck="false" />
        <input v-model="p.value" class="input header-value mono" placeholder="Значение" spellcheck="false" />
        <button class="btn icon-btn" title="Удалить" @click="removeParam(i)"><Icon name="xmark" :size="14" /></button>
      </div>
      <button class="btn" @click="addParam">+ Добавить параметр</button>
    </div>

    <div v-if="activeTab === 'headers'" class="builder-panel">
      <div v-for="(h, i) in headers" :key="i" class="header-row">
        <input v-model="h.name" class="input header-name mono" placeholder="Header" spellcheck="false" />
        <input v-model="h.value" class="input header-value mono" placeholder="Value" spellcheck="false" />
        <button class="btn icon-btn" title="Удалить" @click="removeHeader(i)"><Icon name="xmark" :size="14" /></button>
      </div>
      <button class="btn" @click="addHeader">+ Добавить заголовок</button>
    </div>

    <div v-if="activeTab === 'body'" class="builder-panel">
      <textarea v-model="body" class="textarea body-input" placeholder="{ ... JSON body ... }" spellcheck="false"></textarea>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.builder {
  @apply flex-none border-b border-border bg-bg-panel;
}

.request-bar {
  @apply flex gap-2 items-center px-3 pt-2;
}

.url-field {
  @apply flex-1 flex items-stretch rounded-lg border border-border bg-bg-inset;
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
  @apply flex items-center gap-1 rounded-md border-0 font-semibold text-xs cursor-pointer outline-none;
  padding: 4px 10px;
}

.method-menu {
  @apply absolute top-full left-0 mt-1 z-30 bg-bg-panel border border-border rounded-lg p-1 min-w-24;
  box-shadow: var(--shadow);
}

.method-item {
  @apply block w-full text-left px-2.5 py-1 rounded-md border-0 bg-transparent font-semibold text-xs cursor-pointer;
}

.method-item:hover {
  @apply bg-bg-hover;
}

.url-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none py-0 px-2;
}

.url-input:focus,
.url-input:focus-visible {
  border: 0;
  box-shadow: none;
}

.send-btn {
  @apply flex-none inline-flex items-center justify-center gap-1.5;
}

.send-hint {
  @apply text-[10px] font-medium leading-normal py-0 px-1.5 ml-1 rounded-sm text-white;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.22);
}

.builder-panel {
  @apply mx-3 mb-3 p-2.5 rounded-lg flex flex-col gap-1.5;
  background: var(--bg-inset);
  border: 1px solid var(--border);
}

.header-row {
  @apply flex gap-1.5;
}

.header-name {
  @apply grow-0 shrink-0 basis-2/5;
}

.header-value {
  @apply flex-1;
}

.body-input {
  @apply w-full min-h-35;
}
</style>
