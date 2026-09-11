<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import RawViewer from './RawViewer.vue'
import { copyToClipboard } from '../lib/export'
import { shortcut } from '../lib/platform'

// One text tab: the toolbar and the CodeMirror viewer together. Both the "Raw"
// tab and the body of a non-JSON:API response render this, so a plain response
// looks and behaves the same wherever it is read.
const props = defineProps<{
  text: string
  // The body tab offers to copy a captured request into the request editor.
  showOpenInRequest?: boolean
}>()

const emit = defineEmits<{ (e: 'open-in-request'): void }>()

const searchVisible = ref(false)
const query = ref('')
const searchInput = ref<HTMLInputElement | null>(null)
const viewer = ref<{ next: () => void; prev: () => void } | null>(null)
const stats = ref({ count: 0, index: 0 })
const copied = ref(false)
const searchShortcut = computed(() => shortcut('F'))

function openSearch() {
  searchVisible.value = true
  nextTick(() => searchInput.value?.focus())
}

function closeSearch() {
  searchVisible.value = false
  query.value = ''
}

function onSearchEnter(e: KeyboardEvent) {
  e.preventDefault()
  if (e.shiftKey) viewer.value?.prev()
  else viewer.value?.next()
}

async function copy() {
  if (await copyToClipboard(props.text)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  }
}

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.code === 'KeyF') {
    e.preventDefault()
    openSearch()
  } else if (e.key === 'Escape' && searchVisible.value) {
    closeSearch()
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="toolbar">
    <template v-if="searchVisible">
      <input
        ref="searchInput"
        v-model="query"
        class="input mono flex-1 min-w-0"
        placeholder="Поиск…"
        spellcheck="false"
        @keydown.enter="onSearchEnter"
        @keydown.esc="closeSearch"
      />
      <span class="search-count">
        {{ stats.count ? `${stats.index + 1} / ${stats.count}` : 'нет совпадений' }}
      </span>
      <button class="resp-action icon" title="Предыдущее (Shift+Enter)" @click="viewer?.prev()">
        <Icon name="chevron-up" :size="14" />
      </button>
      <button class="resp-action icon" title="Следующее (Enter)" @click="viewer?.next()">
        <Icon name="chevron-down" :size="14" />
      </button>
      <button class="resp-action icon" title="Закрыть (Esc)" @click="closeSearch">
        <Icon name="xmark" :size="14" />
      </button>
    </template>
    <template v-else>
      <button v-if="showOpenInRequest" class="resp-action open-in-request" @click="emit('open-in-request')">
        Открыть в «Запросе»
      </button>
      <button class="resp-action" @click="copy">
        <Icon v-if="copied" name="check" :size="12" />
        <span>{{ copied ? 'Скопировано' : 'Копировать' }}</span>
      </button>
      <button class="resp-action" @click="openSearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></button>
    </template>
  </div>

  <RawViewer ref="viewer" :text="text" :query="query" @stats="(s) => (stats = s)" />
</template>

<style scoped>
@reference "../style.css";

.open-in-request {
  @apply text-accent;
}
</style>
