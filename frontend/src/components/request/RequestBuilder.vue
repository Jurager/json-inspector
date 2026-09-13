<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import RequestChipPopover from './RequestChipPopover.vue'
import SaveToCollectionSheet from '../collections/SaveToCollectionSheet.vue'
import { Button } from '../ui/button'
import { Popover, PopoverAnchor } from '../ui/popover'
import VarToken from '../ui/VarToken.vue'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '../ui/dropdown-menu'
import { useRequestsStore } from '../../stores/requests'
import { useCollectionsStore } from '../../stores/collections'
import { useEnvironmentsStore } from '../../stores/environments'
import type { ChipName, RequestSource } from '../../lib/requestSource'
import { usePlatform } from '../../composables/usePlatform'
import { registerUrlField } from '../../composables/urlFocus'
import { tokenSegments } from '../../lib/vars'
import { parseRequestCommand, type ParseErrorReason } from '../../lib/parseRequest'
import type { ExportFormat } from '../../lib/export'
import { useToast } from '../../composables/useToast'

// Which request this builder is composing: the command line's, or the card of a saved one. The two
// stores answer the same shape, so nothing below this line has to know which it is.
const props = withDefaults(defineProps<{ source?: 'request' | 'collection' }>(), { source: 'request' })

const requests = useRequestsStore()
const collections = useCollectionsStore()
const store: RequestSource = props.source === 'collection' ? collections : requests

const { shortcut } = usePlatform()
const envStore = useEnvironmentsStore()
const toast = useToast()

const FORMAT_LABELS: Record<ExportFormat, string> = {
  curl: 'cURL',
  fetch: 'fetch',
  wget: 'wget',
  httpie: 'HTTPie',
  powershell: 'PowerShell',
}

const PARSE_ERROR_MESSAGES: Record<ParseErrorReason, string> = {
  'no-url': 'в команде не нашлось ссылки',
  'bad-quotes': 'не закрыта кавычка',
  leftover: 'часть аргументов не разобралась — проверьте флаги команды',
  'bad-fetch-init': 'не разобрался объект настроек fetch',
  'unsupported-variable': 'значение задано переменной PowerShell — взять его негде',
  'unsupported-field': 'поля HTTPie вроде `:=` и `@file` не поддерживаются',
  'unsupported-multipart': 'загрузка файла (multipart) не поддерживается',
}

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

const methodColor = computed(() => METHOD_COLORS[store.method] ?? 'var(--accent)')

const methodBg = computed(() => {
  const c = METHOD_COLORS[store.method] ?? 'var(--accent)'
  return `color-mix(in srgb, ${c} 14%, transparent)`
})

function selectMethod(m: string) {
  void store.setMethod(m)
}

const enabledParamsCount = computed(() => store.enabledParamsCount)
const enabledHeadersCount = computed(() => store.enabledHeadersCount)
const hasBody = computed(() => store.body.trim().length > 0)
// The chip is dashed until the request has code of its own, the way the body chip is: a dashed chip is
// a thing that is not there yet, and it is what the design draws in both places.
const hasScripts = computed(() => Boolean(store.scripts?.pre?.trim() || store.scripts?.post?.trim()))
const isBodyDisabled = computed(() => store.bodyDisabled)

function toggleChip(chip: ChipName) {
  store.setOpenChip(store.openChip === chip ? null : chip)
}

const displayedChip = ref<ChipName | null>(null)
watch(
  () => store.openChip,
  (chip) => {
    if (chip) displayedChip.value = chip
  },
  { immediate: true }
)

const missingVarNames = computed(() => store.missingVars)
const sendBlocked = computed(() => missingVarNames.value.length > 0)

const sendBlockedReason = computed(() =>
  missingVarNames.value.length
    ? `Неизвестные переменные: ${missingVarNames.value.join(', ')}`
    : undefined
)

function createMissing() {
  const envId = envStore.activeId
  if (envId === null) return
  for (const name of missingVarNames.value) envStore.addVar(envId, { name })
  envStore.openSheet({ envId, varName: missingVarNames.value[0] ?? '' })
}

// The request goes out through Go, which is the side that can fill its `{{tokens}}` in — a secret's
// value has not been in this window since it was typed. All the window does is hand over whatever it
// is still holding in its buffers first, so what goes out is what is on screen.
async function send() {
  try {
    await store.send()
  } catch (error) {
    store.failSend()
    toast.show(`Запрос не отправлен: ${String(error)}`, 'error')
  }
}

function cancel() {
  void store.cancel()
}

const urlInputRef = ref<HTMLInputElement | null>(null)

onMounted(() => onBeforeUnmount(registerUrlField(() => urlInputRef.value?.focus())))

const urlDisplayRef = ref<HTMLElement | null>(null)

const urlSegments = computed(() => tokenSegments(store.url))

// Saving into a collection is a gesture of the command line: a card is already in one, and what it
// edits is saved by its own «Сохранить» in the status bar.
const canSave = computed(() => props.source === 'request')

const saveOpen = ref(false)
const saveAnchor = ref<HTMLElement | null>(null)

// The default name is the address the way a person would say it: the last segment, query and all
// the rest left out — a name, not a URL.
const saveDefaultName = computed(() => {
  const raw = store.url.trim()
  if (!raw) return 'Новый запрос'
  try {
    const url = new URL(raw)
    const last = url.pathname.split('/').filter(Boolean).pop()
    return decodeURIComponent(last ?? url.host)
  } catch {
    const withoutQuery = raw.split('?')[0].split('#')[0]
    return withoutQuery.split('/').filter(Boolean).pop() ?? withoutQuery
  }
})
const showUrlDisplay = computed(() => urlSegments.value.length > 0)

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

function onUrlPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text/plain') ?? ''
  const result = parseRequestCommand(text)
  if (result.kind === 'none') return

  e.preventDefault()
  if (result.kind === 'error') {
    toast.show(`Не удалось разобрать команду: ${PARSE_ERROR_MESSAGES[result.reason]}`, 'error')
    return
  }

  // A pasted command is a whole request, not an edit to one: it goes over as a seed and the draft
  // becomes it.
  void store.replace({
    method: result.request.method,
    url: result.request.url,
    headers: Object.entries(result.request.requestHeaders).map(([name, value]) => ({ name, value })),
    body: result.request.requestBody,
    cookies: [],
  })
  store.setOpenChip(null)
  void nextTick(syncUrlScroll)
  toast.show(`Распознан ${FORMAT_LABELS[result.format]}`)
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
                <span>{{ store.method }}</span>
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
            :value="store.url"
            class="url-input mono"
            :class="{ 'url-input-veiled': showUrlDisplay }"
            placeholder="https://api.example.com/articles?include=author"
            spellcheck="false"
            @input="store.setUrl(($event.target as HTMLInputElement).value); syncUrlScroll()"
            @keydown.enter="send"
            @blur="store.flush()"
            @paste="onUrlPaste"
            @scroll="syncUrlScroll"
          />
          <div v-if="showUrlDisplay" ref="urlDisplayRef" class="url-display mono" aria-hidden="true">
            <template v-for="(seg, i) in urlSegments" :key="i">
              <VarToken v-if="seg.tokenName" :name="seg.tokenName" :offset="seg.start" />
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
              :title="isBodyDisabled ? `${store.method} не отправляет тело` : undefined"
              @click="toggleChip('body')"
            >
              Тело
            </button>
            <button
              class="chip chip-body chip-scripts"
              :class="{ 'has-body': hasScripts, active: store.openChip === 'scripts' }"
              @click="toggleChip('scripts')"
            >
              <Icon name="code-xml" :size="10" />
              Скрипты
            </button>
          </PopoverAnchor>
          <RequestChipPopover v-if="displayedChip" :chip="displayedChip" :source="store" />
        </Popover>
      </div>

      <span ref="saveAnchor" class="save-anchor">
        <button
          class="bookmark-btn"
          :disabled="!canSave || !store.url.trim()"
          :title="canSave ? 'Сохранить в коллекцию' : 'Запрос уже в коллекции'"
          @click="saveOpen = !saveOpen"
        >
          <Icon name="bookmark" :size="14" />
        </button>
        <SaveToCollectionSheet
          v-if="canSave && saveAnchor"
          :open="saveOpen"
          :url="store.url"
          :default-name="saveDefaultName"
          @update:open="saveOpen = $event"
        />
      </span>

      <Button
        variant="primary"
        size="lg"
        :disabled="!store.url.trim() || sendBlocked"
        :title="sendBlockedReason"
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
          <span class="missing-name mono" v-for="n in missingVarNames" :key="n">{{ n }}</span>
          не подставляются. Отправка заблокирована.
        </template>
        <template v-else>
          В окружении <b>{{ envStore.activeEnvironment?.name }}</b> нет
          {{ missingVarNames.length === 1 ? 'переменной' : 'переменных' }}:
          <span class="missing-name mono" v-for="n in missingVarNames" :key="n">{{ n }}</span>
          Отправка заблокирована.
        </template>
      </span>
      <Button
        variant="primary"
        :disabled="envStore.activeId === null"
        :title="envStore.activeId === null ? 'Сначала выберите окружение в шапке' : undefined"
        @click="createMissing"
      >
        {{ missingVarNames.length === 1 ? 'Создать' : 'Создать все' }}
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
  @apply relative flex items-center gap-2 h-12 py-3 px-4;
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
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none px-2 text-[13px];
  font-family: var(--mono);
  color: var(--text);
}

.url-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

.url-display {
  @apply absolute inset-0 flex items-center overflow-hidden px-2 text-[13px] pointer-events-none;
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

/* The code icon is smaller than the text beside it and sits on its baseline. */
.chip-scripts :deep(svg) {
  flex: none;
}

.chip-body:disabled {
  @apply opacity-50 cursor-default;
}

.save-anchor {
  @apply relative flex-none;
}

.bookmark-btn {
  @apply flex-none w-8 h-8 rounded-lg flex items-center justify-center text-accent bg-bg-panel cursor-pointer;
  border: 1px solid var(--border-strong);
  --wails-draggable: no-drag;
}

.bookmark-btn:hover:not(:disabled) {
  background: var(--accent-soft);
}

.bookmark-btn:disabled {
  @apply opacity-50 cursor-default;
}

.send-hint {
  @apply text-[10px] font-medium leading-normal py-0 px-1.5 ml-1 rounded-sm text-white;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.22);
}
</style>
