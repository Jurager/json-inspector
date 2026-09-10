<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { SendRequest, CancelRequest } from '../../wailsjs/go/main/App'
import { useRequestsStore } from '../stores/requests'
import { buildSampleRecord } from '../lib/sample'
import { shortcut } from '../lib/platform'

const store = useRequestsStore()

const sendShortcut = computed(() => shortcut('↵'))

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

const method = ref('GET')
const url = ref('')
const body = ref('')
type BuilderPanel = 'headers' | 'body'
const openPanel = ref<BuilderPanel | null>(null)

function togglePanel(panel: BuilderPanel) {
  openPanel.value = openPanel.value === panel ? null : panel
}

interface HeaderRow {
  name: string
  value: string
}

const headers = ref<HeaderRow[]>([
  { name: 'Accept', value: 'application/vnd.api+json' },
])

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

async function send() {
  if (!url.value.trim() || store.loading) return
  store.loading = true
  const requestHeaders = collectHeaders()
  try {
    const res = await SendRequest(method.value, url.value.trim(), requestHeaders, body.value)
    if (res.cancelled) return
    store.add({
      method: method.value,
      url: url.value.trim(),
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

function loadSample() {
  store.add(buildSampleRecord())
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
    <div class="builder-row">
      <select v-model="method" class="select method-select">
        <option v-for="m in METHODS" :key="m" :value="m">{{ m }}</option>
      </select>
      <input
        v-model="url"
        class="input url-input mono"
        placeholder="https://api.example.com/articles?include=author"
        spellcheck="false"
        @keydown.enter="send"
      />
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
      <button class="btn" title="Загрузить пример JSON:API" @click="loadSample">Образец</button>
    </div>

    <div class="builder-options">
      <button
        class="opt-toggle"
        :class="{ active: openPanel === 'headers' }"
        @click="togglePanel('headers')"
      >
        Заголовки ({{ headers.filter((h) => h.name.trim()).length }})
      </button>
      <button
        class="opt-toggle"
        :class="{ active: openPanel === 'body' }"
        @click="togglePanel('body')"
      >
        Тело
      </button>
    </div>

    <div v-if="openPanel === 'headers'" class="builder-panel">
      <div v-for="(h, i) in headers" :key="i" class="header-row">
        <input v-model="h.name" class="input header-name mono" placeholder="Header" spellcheck="false" />
        <input v-model="h.value" class="input header-value mono" placeholder="Value" spellcheck="false" />
        <button class="btn icon-btn" title="Удалить" @click="removeHeader(i)"><Icon name="xmark" :size="14" /></button>
      </div>
      <button class="btn" @click="addHeader">+ Добавить заголовок</button>
    </div>

    <div v-if="openPanel === 'body'" class="builder-panel">
      <textarea v-model="body" class="textarea body-input" placeholder="{ ... JSON body ... }" spellcheck="false"></textarea>
    </div>
  </div>
</template>

<style scoped>
@reference "../style.css";

.builder {
  @apply flex-none border-b border-border bg-bg-panel p-3;
}

.builder-row {
  @apply flex gap-2 items-center;
}

.method-select {
  @apply flex-none font-semibold text-accent;
}

.url-input {
  @apply flex-1 min-w-0;
}

.builder-options {
  @apply flex gap-1 mt-2.5;
}

.opt-toggle {
  @apply border-none bg-transparent text-text-secondary text-xs py-1 px-2 rounded-md cursor-pointer;
}

.opt-toggle:hover {
  @apply bg-bg-hover text-text;
}

.opt-toggle.active {
  @apply bg-accent-soft text-accent;
}

.builder-panel {
  @apply mt-2 p-2.5 rounded-lg flex flex-col gap-1.5;
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

.send-btn {
  @apply min-w-22 inline-flex items-center justify-center;
}

.send-hint {
  @apply text-[10px] font-medium leading-normal py-0 px-1.5 ml-1.5 rounded-sm text-white;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.22);
}
</style>
