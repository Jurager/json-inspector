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
import { looksLikeCommand } from '../../lib/commandShape'
import { AuthType } from '../../../bindings/json-inspector/internal/domain'
import { CommandKind } from '../../../bindings/json-inspector/internal/usecase/draft'
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
// A section with nothing in it is a quiet label rather than a dashed box: the emptiness is the ink of
// the word, and a frame around it would be a shape to read for something that is not there.
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

// Names stop missing when the environments change, and that happens in another window: the draft is
// not edited, so nothing here would hear about it. Adding a variable, renaming one, switching the
// active environment — each is a new snapshot, and each is a reason to ask again.
watch(
  () => envStore.envState,
  () => void store.refreshPreview()
)

const sendBlockedReason = computed(() =>
  missingVarNames.value.length
    ? t('request.missingBlocked', { names: missingVarNames.value.join(', ') })
    : undefined
)

// A read-only environment is one this window does not write into — that is what its Access switch
// is for — so a button that quietly put a variable inside it would be breaking the window's own
// rule. Instead it opens that environment, where the switch is, and says what is in the way.
const activeReadonly = computed(() => Boolean(envStore.activeEnvironment?.readonly))

const createLabel = computed(() =>
  missingVarNames.value.length === 1 ? t('request.createVar') : t('request.createVarAll')
)

// The line is one sentence and it truncates rather than wrapping, so the whole of it travels in the
// tooltip — where it is plain text, and can say what the pieces of the sentence say in words.
const missingTitle = computed(() => {
  const names = missingVarNames.value.join(', ')
  const env = envStore.activeEnvironment?.name
  return env
    ? t('request.missingTitle', { n: missingVarNames.value.length, names, env })
    : t('request.missingNoEnvTitle', { n: missingVarNames.value.length, names })
})

const createTitle = computed(() => {
  if (envStore.activeId === null) return t('request.chooseEnvFirst')
  if (activeReadonly.value) return t('request.envReadOnly')
  return undefined
})

function createMissing() {
  const envId = envStore.activeId
  if (envId === null) return
  if (!activeReadonly.value) {
    for (const name of missingVarNames.value) {
      void envStore.addVar(envId, { name }).catch((error) => {
        toast.show(t('request.createFailed', { error: describeFailure(error) }), 'error')
      })
    }
  }
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

// Which environment the request will be sent with, named in the bar rather than only in the window's
// own chrome: it is the last thing a person checks before pressing send, and it belongs beside the
// request it applies to.
const envName = computed(() => envStore.activeEnvironment?.name ?? '')

// Whether a section holds anything, which is what tells a filled segment from an empty one. "None"
// and "inherit" are answers about who authorizes the request rather than credentials, so they are the
// two the authorization section is empty in.
function chipFilled(chip: ChipName): boolean {
  switch (chip) {
    case 'params':
      return enabledParamsCount.value > 0
    case 'headers':
      return enabledHeadersCount.value > 0
    case 'auth':
      return store.auth.type !== AuthType.AuthNone && store.auth.type !== AuthType.AuthInherit
    case 'body':
      return hasBody.value
    case 'scripts':
      return hasScripts.value
  }
}

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

// A paste is read on the other side, which is where the parsing lives — quoting dialects are logic,
// not drawing. What stays here is the decision the browser forces: `preventDefault` has to be
// called before any answer could arrive, so this has to know whether the paste is worth
// interrupting. That is the whole of what looksLikeCommand does, and it is allowed to be wrong: an
// address pasted into the address field — the common case — is never intercepted, and a text that
// slips through as prose is pasted by the browser as text.
async function onUrlPaste(e: ClipboardEvent) {
  const input = e.target as HTMLInputElement
  const text = e.clipboardData?.getData('text/plain') ?? ''
  if (!looksLikeCommand(text)) return

  e.preventDefault()
  const reading = await store.pasteCommand(text)

  if (reading.kind === CommandKind.KindError) {
    toast.show(t('request.pasteFailed', { reason: t(`request.parseError.${reading.reason}`) }), 'error')
    return
  }
  if (reading.kind !== CommandKind.KindOK) {
    // It looked like a command and turned out not to be: the paste the browser was not allowed to
    // make itself, made here.
    insertAtCaret(input, text)
    return
  }

  store.setOpenChip(null)
  void nextTick(syncUrlScroll)
  toast.show(t('request.pasteRecognised'))
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
  <div class="request-block">
    <div ref="requestBarEl" class="request-bar">
      <div class="bar-row">
        <div class="url-field" :class="{ 'url-field-invalid': sendBlocked }">
          <!-- The verb is the field's own left segment rather than a control beside it: which method a
               request is stands inside the address it is sent to, and a button outside the box would be
               a second place to look for one thing. -->
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <button class="method-plate method-btn" :style="{ color: methodColor, background: methodBg }">
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

          <!-- The address is one box drawn twice: the input a person types into, and the painted
               copy under it that shows the tokens and the query in their own colours. Only the copy
               is ever seen. -->
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
            <div
              v-if="showUrlDisplay"
              ref="urlDisplayRef"
              class="url-display mono"
              aria-hidden="true"
            >
              <template v-for="(seg, i) in urlSegments" :key="i">
                <VarToken
                  v-if="seg.tokenName"
                  :name="seg.tokenName"
                  :text="seg.text"
                  :offset="seg.start"
                />
                <span v-else>{{ seg.text }}</span>
              </template>
            </div>
          </div>
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
          class="send-btn"
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

      <!-- The sections of the request, as a segmented control rather than a row of pills: a filled
           section and an empty one are told apart by the ink of the label and the number beside it,
           not by a badge, and the open one is the raised pill inside the track. -->
      <div class="bar-row segments-row">
        <Popover :open="store.openChip !== null" @update:open="(v) => !v && store.setOpenChip(null)">
          <PopoverAnchor class="segments">
            <button
              class="segment"
              :class="{ active: store.openChip === 'params', filled: chipFilled('params') }"
              @click="toggleChip('params')"
            >
              <span>{{ t('request.chips.params') }}</span>
              <span v-if="enabledParamsCount" class="segment-count">{{ enabledParamsCount }}</span>
            </button>
            <button
              class="segment"
              :class="{ active: store.openChip === 'headers', filled: chipFilled('headers') }"
              @click="toggleChip('headers')"
            >
              <span>{{ t('request.chips.headers') }}</span>
              <span v-if="enabledHeadersCount" class="segment-count">{{ enabledHeadersCount }}</span>
            </button>
            <button
              class="segment"
              :class="{ active: store.openChip === 'auth', filled: chipFilled('auth') }"
              @click="toggleChip('auth')"
            >
              <span>{{ t('request.chips.auth') }}</span>
            </button>
            <button
              class="segment"
              :class="{ active: store.openChip === 'body', filled: chipFilled('body') }"
              @click="toggleChip('body')"
            >
              <span>{{ t('request.chips.body') }}</span>
            </button>
            <button
              class="segment"
              :class="{ active: store.openChip === 'scripts', filled: chipFilled('scripts') }"
              @click="toggleChip('scripts')"
            >
              <span>{{ t('request.chips.scripts') }}</span>
            </button>
          </PopoverAnchor>
          <RequestChipPopover v-if="displayedChip" :chip="displayedChip" :source="store" />
        </Popover>

        <div class="bar-spacer"></div>

        <!-- The environment this request will be sent with: the bar is where a person looks before
             pressing send, and the name belongs beside the thing it applies to. -->
        <span v-if="envName" class="env-name">{{ envName }}</span>
      </div>
    </div>

    <!-- The bar is the window's own row rather than a card in the column: it says what is holding
         the send back, and it stands where the request ends and the answer begins. The sentence
         carries the names in place, so it is written on one line — the whitespace between the pieces
         is part of it. -->
    <div v-if="sendBlocked" class="var-error">

      <span v-if="envStore.activeId === null" class="var-error-text" :title="missingTitle">{{ t('request.missingVariable', missingVarNames.length) }} <span v-for="(n, i) in missingVarNames" :key="n" class="var-name mono">{{ n }}<span v-if="i < missingVarNames.length - 1">, </span></span> {{ t('request.missingNoEnv') }}</span>
      <span v-else class="var-error-text" :title="missingTitle">{{ t('request.missingVariable', missingVarNames.length) }} <span v-for="(n, i) in missingVarNames" :key="n" class="var-name mono">{{ n }}<span v-if="i < missingVarNames.length - 1">, </span></span> {{ t('request.missingNotIn', missingVarNames.length) }} <b>{{ envStore.activeEnvironment?.name }}</b> {{ t('request.missingSendingBlocked') }}</span>

      <!-- One way out, as the drawing gives it. In an environment the user has closed the button is
           shut and its tooltip says what opens it — the sentence already names the environment, so
           the two read together. -->
      <button
        type="button"
        class="var-error-action"
        :disabled="envStore.activeId === null || activeReadonly"
        :title="createTitle"
        @click="createMissing"
      >
        {{ createLabel }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The drawing's own row: full width, the soft red of a warning, and a hairline under it that
   separates the request from the answer rather than ringing the notice as a card. */
.var-error {
  @apply flex-none flex items-center gap-2.5 py-2.5 px-5 border-b;
  background: var(--red-soft);
  border-color: var(--border);
}

/* One sentence, and it gives way rather than wrapping: a long address in a tooltip is better than a
   second line that pushes the answer down. */
.var-error-text {
  @apply flex-1 min-w-0 text-[13px] text-text;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.var-error-text b {
  @apply font-semibold;
}

/* The names are values in a sentence about them, so they are drawn as values: the mono face (the
   `mono` class the markup wears) and the colour of the thing that is wrong, not a chip of their own. */
.var-name {
  @apply text-[13px] font-semibold;
  color: var(--red-text);
}

.var-error-action {
  @apply flex-none h-[30px] px-3.5 border-0 rounded-[7px] cursor-pointer whitespace-nowrap
         text-accent-text text-[13px] font-semibold;
  font-family: inherit;
  background: var(--accent);
}

.var-error-action:hover:not(:disabled) {
  @apply brightness-110;
}

.var-error-action:disabled {
  @apply opacity-50 cursor-default;
}

/* What makes the shared plate a control here: the pointer, and the room for the chevron beside the
   verb. The plate's own box and ink live in style.css, where the browser's inert twin reads them. */
.method-btn {
  @apply border-0 cursor-pointer outline-none;
  --wails-draggable: no-drag;
}

.url-text {
  /* The address keeps a floor of its own: it is the one thing in this row that cannot be guessed from
     anything else, so when the window is too narrow the save button gives way before it does. */
  @apply relative flex-1 flex items-center min-w-0;
  min-width: 120px;
}

.url-input {
  @apply flex-1 min-w-0 bg-transparent border-0 outline-none;
  padding: 0;
  font-family: var(--mono);
  font-size: 13px;
  color: var(--text);
}

.url-input-veiled {
  color: transparent;
  caret-color: var(--text);
}

.url-display {
  @apply absolute inset-0 flex items-center overflow-hidden pointer-events-none;
  padding: 0;
  font-family: var(--mono);
  font-size: 13px;
  white-space: pre;
  color: var(--text);
}

.url-input:focus,
.url-input:focus-visible {
  border: 0;
  box-shadow: none;
}

.save-anchor {
  @apply relative flex-none;
}

/* The save button is the field's own height and nothing else: no frame, no fill, and the accent only
   when the pointer is on it — it is the quietest of the three things in this row. */
.bookmark-btn {
  @apply flex-none flex items-center justify-center cursor-pointer;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-secondary);
  --wails-draggable: no-drag;
}

.bookmark-btn:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--accent);
}

.bookmark-btn:disabled {
  @apply opacity-50 cursor-default;
}

/* The send button is a size of this bar's own rather than of the shared control: the field beside it
   is 34 tall, and the two are the same object seen twice. Named with the primitive's class as well,
   because a scoped override and the primitive's own scoped rule have the same specificity. */
.btn.send-btn {
  @apply flex-none;
  height: 34px;
  padding: 0 15px;
  border-radius: 7px;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
}

.send-hint {
  @apply font-medium;
  font-family: var(--mono);
  font-size: 11px;
  opacity: 0.75;
}
</style>
