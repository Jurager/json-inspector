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
import { CommandService } from '../../../bindings/json-inspector/internal/transport/wails'
import { Format, Kind } from '../../../bindings/json-inspector/internal/command'
import { useToast } from '../../composables/useToast'
import { describeFailure, useMessages } from '../../i18n'

// Which request this builder is composing: the command line's, or the card of a saved one. The two
// stores answer the same shape, so nothing below this line has to know which it is.
const props = withDefaults(defineProps<{ source?: 'request' | 'collection' }>(), { source: 'request' })

const { t } = useMessages()

const requests = useRequestsStore()
const collections = useCollectionsStore()
const store: RequestSource = props.source === 'collection' ? collections : requests

const { shortcut } = usePlatform()
const envStore = useEnvironmentsStore()
const toast = useToast()

// Wire formats, not words: a cURL command is called cURL in every language. An index signature
// rather than Record<Format, …>: the enum's `$zero` is not a format, and the map is only ever read
// with one that is.
const FORMAT_LABELS: { [format: string]: string } = {
  [Format.FormatCurl]: 'cURL',
  [Format.FormatFetch]: 'fetch',
  [Format.FormatWget]: 'wget',
  [Format.FormatHTTPie]: 'HTTPie',
  [Format.FormatPowerShell]: 'PowerShell',
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
// A body is text, a form with a row in it, or a file — the store knows which, and both stores know
// it the same way.
const hasBody = computed(() => store.hasBody)
// The chip is dashed until the request has code of its own, the way the body chip is: a dashed chip is
// a thing that is not there yet, and it is what the design draws in both places.
const hasScripts = computed(() => Boolean(store.scripts?.pre?.trim() || store.scripts?.post?.trim()))

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
    ? t('request.missingBlocked', { names: missingVarNames.value.join(', ') })
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
    toast.show(t('request.sendFailed', { error: describeFailure(error) }), 'error')
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
  if (!raw) return t('request.newRequestName')
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

// A paste is read by the parser in Go, and the answer arrives after the browser has already decided
// what to do with the event — so the paste is taken from it up front and, when the text turns out
// not to be a command at all, put into the field here. The alternative is deciding in this window
// whether the text is a command, which is the parser over again.
async function onUrlPaste(e: ClipboardEvent) {
  const input = e.target as HTMLInputElement
  const text = e.clipboardData?.getData('text/plain') ?? ''
  e.preventDefault()

  let result
  try {
    result = await CommandService.Parse(text)
  } catch {
    insertAtCaret(input, text)
    return
  }

  if (result.kind === Kind.KindNone) {
    insertAtCaret(input, text)
    return
  }
  if (result.kind === Kind.KindError) {
    toast.show(t('request.pasteFailed', { reason: t(`request.parseError.${result.reason}`) }), 'error')
    return
  }

  // A pasted command is a whole request, not an edit to one: it goes over as a seed and the draft
  // becomes it. The credential goes as the scheme it is, so the Auth chip shows what the command
  // asked for and the answers stay editable — a header somebody wrote by hand stays a header.
  void store.replace({
    method: result.request.method,
    url: result.request.url,
    headers: (result.request.headers ?? []).map((h) => ({ name: h.name, value: h.value })),
    body: result.request.body,
    cookies: [],
    auth: result.request.auth ?? undefined,
  })
  store.setOpenChip(null)
  void nextTick(syncUrlScroll)
  toast.show(t('request.pasteRecognised', { format: FORMAT_LABELS[result.format] }))
}

// insertAtCaret is the paste the browser was not allowed to make itself: the same text where the
// caret was, with an input event so the store hears about it. `end` puts the caret after it, which
// is where a paste leaves it.
function insertAtCaret(input: HTMLInputElement, text: string) {
  const from = input.selectionStart ?? input.value.length
  const to = input.selectionEnd ?? from
  input.setRangeText(text, from, to, 'end')
  input.dispatchEvent(new Event('input', { bubbles: true }))
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
              {{ t('request.chips.params') }} <span v-if="enabledParamsCount" class="chip-count">{{ enabledParamsCount }}</span>
            </button>
            <button class="chip" :class="{ active: store.openChip === 'headers' }" @click="toggleChip('headers')">
              {{ t('request.chips.headers') }} <span v-if="enabledHeadersCount" class="chip-count">{{ enabledHeadersCount }}</span>
            </button>
            <button class="chip" :class="{ active: store.openChip === 'auth' }" @click="toggleChip('auth')">{{ t('request.chips.auth') }}</button>
            <button
              class="chip chip-body"
              :class="{ 'has-body': hasBody, active: store.openChip === 'body' }"
              @click="toggleChip('body')"
            >
              {{ t('request.chips.body') }}
            </button>
            <button
              class="chip chip-body chip-scripts"
              :class="{ 'has-body': hasScripts, active: store.openChip === 'scripts' }"
              @click="toggleChip('scripts')"
            >
              <Icon name="code-xml" :size="10" />
              {{ t('request.chips.scripts') }}
            </button>
          </PopoverAnchor>
          <RequestChipPopover v-if="displayedChip" :chip="displayedChip" :source="store" />
        </Popover>
      </div>

      <span v-if="canSave" ref="saveAnchor" class="save-anchor">
        <button
          class="bookmark-btn"
          :disabled="!store.url.trim()"
          :title="t('request.saveToCollection')"
          @click="saveOpen = !saveOpen"
        >
          <Icon name="bookmark" :size="14" />
        </button>
        <SaveToCollectionSheet
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
          <span>{{ t('common.cancel') }}</span>
        </template>
        <template v-else>
          <span>{{ t('request.send') }}</span>
          <kbd class="send-hint">{{ sendShortcut }}</kbd>
        </template>
      </Button>

    </div>

    <div v-if="sendBlocked" class="missing-row">
      <span class="missing-text">
        <template v-if="envStore.activeId === null">
          {{ t('request.missingNoEnvHead') }}
          <span class="missing-name mono" v-for="n in missingVarNames" :key="n">{{ n }}</span>
          {{ t('request.missingNoEnvTail') }}
        </template>
        <template v-else>
          {{ t('request.missingInEnvHead', { name: envStore.activeEnvironment?.name }) }}
          {{ t('request.missingInEnvCount', missingVarNames.length) }}:
          <span class="missing-name mono" v-for="n in missingVarNames" :key="n">{{ n }}</span>
          {{ t('request.missingInEnvTail') }}
        </template>
      </span>
      <Button
        variant="primary"
        :disabled="envStore.activeId === null"
        :title="envStore.activeId === null ? t('request.chooseEnvFirst') : undefined"
        @click="createMissing"
      >
        {{ missingVarNames.length === 1 ? t('request.createVar') : t('request.createVarAll') }}
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
  /* The field is what gives way first when the window narrows, and its floor is where the URL stays
     readable. Without a basis a flex item refuses to shrink past its own content, and the row pushes
     the chips and the send button off the right edge — which is what a narrow window used to do. */
  @apply flex items-stretch rounded-lg border border-border bg-bg-inset h-8 overflow-hidden;
  flex: 1 1 260px;
  min-width: 140px;
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
  /* The address keeps a floor of its own: it is the one thing in this row that cannot be guessed from
     anything else, so when the window is too narrow the chip strip gives way before it does. */
  @apply relative flex-1 flex items-stretch;
  min-width: 120px;
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
