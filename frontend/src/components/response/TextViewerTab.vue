<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '../ui/Icon.vue'
import { Button, IconButton } from '../ui/button'
import { Input } from '../ui/input'
import RawViewer from './RawViewer.vue'
import { copyToClipboard } from '../../lib/clipboard'
import { usePlatform } from '../../composables/usePlatform'

// Shared by the "Raw" tab and the body of a non-JSON:API response, so a plain
// response reads the same wherever it is shown.
const { shortcut } = usePlatform()

const props = defineProps<{
  text: string
  showOpenInRequest?: boolean
}>()

const emit = defineEmits<{ (e: 'open-in-request'): void }>()

const searchVisible = ref(false)
const query = ref('')
const searchInput = ref<InstanceType<typeof Input> | null>(null)
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
      <Input
        ref="searchInput"
        v-model="query"
        mono
        class="flex-1 min-w-0"
        placeholder="Поиск…"
        spellcheck="false"
        @keydown.enter="onSearchEnter"
        @keydown.esc="closeSearch"
      />
      <span class="search-count">
        {{ stats.count ? `${stats.index + 1} / ${stats.count}` : 'нет совпадений' }}
      </span>
      <IconButton variant="outline" hint="Предыдущее (Shift+Enter)" @click="viewer?.prev()">
        <Icon name="chevron-up" :size="14" />
      </IconButton>
      <IconButton variant="outline" hint="Следующее (Enter)" @click="viewer?.next()">
        <Icon name="chevron-down" :size="14" />
      </IconButton>
      <IconButton variant="outline" hint="Закрыть (Esc)" @click="closeSearch">
        <Icon name="xmark" :size="14" />
      </IconButton>
    </template>
    <template v-else>
      <Button v-if="showOpenInRequest" size="sm" class="open-in-request" @click="emit('open-in-request')">
        Открыть в «Запросе»
      </Button>
      <Button size="sm" @click="copy">
        <Icon v-if="copied" name="check" :size="12" />
        <span>{{ copied ? 'Скопировано' : 'Копировать' }}</span>
      </Button>
      <Button size="sm" @click="openSearch"><span>Поиск</span><kbd class="keycap">{{ searchShortcut }}</kbd></Button>
    </template>
  </div>

  <RawViewer ref="viewer" :text="text" :query="query" @stats="(s) => (stats = s)" />
</template>

<style scoped>
@reference "../../style.css";

/* Names .btn so it outranks the colour the primitive sets on its own root. */
.btn.open-in-request {
  @apply text-accent;
}
</style>
