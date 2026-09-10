<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { SendRequest } from '../../wailsjs/go/main/App'
import { useRequestsStore } from '../stores/requests'
import { buildSampleRecord } from '../lib/sample'

const store = useRequestsStore()

const shortcut = /Mac/i.test(navigator.userAgent) ? '⌘↵' : 'Ctrl+↵'

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
      <button class="btn btn-primary send-btn" :disabled="store.loading || !url.trim()" @click="send">
        <span v-if="store.loading" class="spinner"></span>
        <template v-else>
          <span>Отправить</span>
          <kbd class="send-hint">{{ shortcut }}</kbd>
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
.builder {
  flex: 0 0 auto;
  border-bottom: 1px solid var(--border);
  background: var(--bg-panel);
  padding: 12px;
}

.builder-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.method-select {
  flex: 0 0 auto;
  font-weight: 600;
  color: var(--accent);
}

.url-input {
  flex: 1;
  min-width: 0;
}

.builder-options {
  display: flex;
  gap: 4px;
  margin-top: 10px;
}

.opt-toggle {
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
}

.opt-toggle:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.opt-toggle.active {
  background: var(--accent-soft);
  color: var(--accent);
}

.builder-panel {
  margin-top: 8px;
  padding: 10px;
  background: var(--bg-inset);
  border: 1px solid var(--border);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.header-row {
  display: flex;
  gap: 6px;
}

.header-name {
  flex: 0 0 40%;
}

.header-value {
  flex: 1;
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 10px;
  line-height: 1;
}

.body-input {
  width: 100%;
  min-height: 140px;
}

.send-btn {
  min-width: 88px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.send-hint {
  font-family: inherit;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.5;
  padding: 0 5px;
  margin-left: 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.22);
  color: #fff;
}
</style>
