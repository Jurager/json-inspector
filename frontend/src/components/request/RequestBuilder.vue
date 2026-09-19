<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from '../ui/Icon.vue'
import BarEnvironment from './BarEnvironment.vue'
import BarSections from './BarSections.vue'
import SaveRequestPopover from '../collections/SaveRequestPopover.vue'
import { Button } from '../ui/button'
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
import type { RequestSource } from '../../lib/requestSource'
import { usePlatform } from '../../composables/usePlatform'
import { registerUrlField } from '../../composables/urlFocus'
import { useRequestEnvironment } from '../../composables/useRequestEnvironment'
import { urlPieces } from '../../lib/vars'
import { collectionPlaces } from '../../lib/collectionPlaces'
import { looksLikeCommand } from '../../lib/commandShape'
import { CommandKind } from '../../../bindings/json-inspector/internal/usecase/draft'
import { useToast } from '../../composables/useToast'
import { describeFailure, formatNumber, useMessages } from '../../i18n'

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

const missingVarNames = computed(() => store.missingVars)
const sendBlocked = computed(() => missingVarNames.value.length > 0)

// Names stop missing when the environments change, and that happens in another window: the draft is
// not edited, so nothing here would hear about it. Adding a variable, renaming one, switching the
// active environment — each is a new snapshot, and each is a reason to ask again. A pinned request
// asks the same question, so its pin belongs in the list of reasons.
watch(
  () => [envStore.envState, store.environmentId],
  () => void store.refreshPreview()
)

const sendBlockedReason = computed(() => {
  if (!sendBlocked.value) return undefined
  const names = missingVarNames.value.join(', ')
  const env = answersIn.value
  return env
    ? t('request.missingTitle', { n: missingVarNames.value.length, names, env })
    : t('request.missingNoEnvTitle', { n: missingVarNames.value.length, names })
})

// Which environment this request answers in — the window's, or one it pinned — and where a variable
// written for it belongs. Both come from one place: the sentence names an environment, and the button
// beside it has to put the variable in the one it named.
const { answersIn, stale, scopeId, scopeReadonly } = useRequestEnvironment(store)

// A read-only environment is one this window does not write into — that is what its Access switch is
// for — so a button that quietly put a variable inside it would be breaking the window's own rule.
// Instead it opens that environment, where the switch is, and says what is in the way.
function createMissing() {
  const envId = scopeId.value
  if (envId === null) return
  if (!scopeReadonly.value) {
    for (const name of missingVarNames.value) {
      void envStore.addVar(envId, { name }).catch((error) => {
        toast.show(t('request.createFailed', { error: describeFailure(error) }), 'error')
      })
    }
  }
  envStore.openSheet({ envId, varName: missingVarNames.value[0] ?? '' })
}

const createLabel = computed(() =>
  missingVarNames.value.length === 1 ? t('request.createVar') : t('request.createVarAll')
)

const createTitle = computed(() => {
  // The three reasons the button is shut, and each one says what would open it: an environment this
  // request is pointed at and that is not there any more is not the same as having none at all.
  if (stale.value) return t('request.pinGone')
  if (scopeId.value === null) return t('request.chooseEnvFirst')
  if (scopeReadonly.value) return t('request.envReadOnly')
  return undefined
})

// ── The send button ──────────────────────────────────────────────────────────────────────────────

// How long the request has been in flight, in tenths of a second: a send that takes a while has to
// look like something happening rather than something stuck.
const elapsed = ref(0)
let ticker: number | null = null

function stopTicker() {
  if (ticker !== null) {
    clearInterval(ticker)
    ticker = null
  }
}

watch(
  () => store.loading,
  (loading) => {
    stopTicker()
    if (!loading) return
    elapsed.value = 0
    ticker = window.setInterval(() => (elapsed.value += 1), 100)
  }
)

onBeforeUnmount(stopTicker)

const sendLabel = computed(() => (store.loading ? t('common.cancel') : t('request.send')))
const sendKey = computed(() =>
  store.loading ? `${formatNumber(elapsed.value / 10, 1)} ${t('units.s')}` : sendShortcut.value
)
// While it is in flight the button is the way to stop it, so it stays live whatever else is wrong:
// the address it was sent to is no longer the question.
const sendDisabled = computed(() => !store.loading && (!store.url.trim() || sendBlocked.value))

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

function onSend() {
  if (store.loading) cancel()
  else void send()
}

const urlInputRef = ref<HTMLInputElement | null>(null)

onMounted(() => onBeforeUnmount(registerUrlField(() => urlInputRef.value?.focus())))

const urlDisplayRef = ref<HTMLElement | null>(null)

const urlParts = computed(() => urlPieces(store.url))
const showUrlDisplay = computed(() => urlParts.value.length > 0)

// Saving into a collection is a gesture of the command line: a card is already in one, and what it
// edits is saved by its own «Сохранить» in the status bar.
const canSave = computed(() => props.source === 'request')

const saveOpen = ref(false)
const saveAnchor = ref<HTMLElement | null>(null)
// The panel's own key goes to the panel: what the shortcut does with it up is the panel's to say.
const savePanel = ref<InstanceType<typeof SaveRequestPopover> | null>(null)

// A dot on the bookmark while the line holds something a collection does not: it is there for a
// request somebody typed, and it goes once the request is kept — saved, or sent and filed in the
// history — and comes back on the next edit of it.
//
// What is being typed counts before Go has heard of it. The revision the store counts is Go's, and
// its answer to a keystroke is a round trip away, so a dot that waited for it would lag every line
// by the length of a flush.
//
// The question is the command line's own and so is the answer: a card is drawn by the same builder
// but kept by its own button in the status bar, so it never wears this dot.
// Whether there is anything to keep at all: saving is the command line's gesture, and a line with no
// address is not a request yet.
const canSaveNow = computed(() => canSave.value && Boolean(requests.url.trim()))

const unsaved = computed(
  () =>
    canSaveNow.value &&
    (requests.lineDirty || requests.bufferedUrl || requests.bufferedBody)
)
const saveHint = computed(() => (unsaved.value ? t('request.saveHint') : t('request.saveToCollection')))

// Where a save could go, which is what tells the shortcut that the place it remembers is still
// there. The sheet draws the same list with the tree's names on it.
const places = computed(() => collectionPlaces(collections.tree))

// What the request would be called: the address without its scheme, which is what tells one row of a
// collection from another. The verb is left out on purpose — the same endpoint is usually written to
// in a collection that reads it, and two rows that differ by nothing else are two rows nobody can
// tell apart. The panel opens its name field with this in it, and the name is theirs to change.
const saveDefaultName = computed(() => {
  const raw = store.url.trim()
  if (!raw) return t('request.newRequestName')
  return raw.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '')
})

function syncUrlScroll() {
  const input = urlInputRef.value
  const display = urlDisplayRef.value
  if (input && display) display.scrollLeft = input.scrollLeft
}

function onWindowKeydown(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey)) return
  // By code and not by key: the layouts the window is used in put other letters on this key, and a
  // shortcut that only answers on one of them is a shortcut half the users do not have.
  if (e.code === 'KeyS') {
    // A card is kept by its own button, and a line with no address is not a request yet: the press
    // is left to the window rather than swallowed by a shortcut with nothing to do.
    if (!canSaveNow.value) return
    e.preventDefault()
    void saveShortcut()
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    void send()
  }
}

// What the sheet promises in its own last line: the next request goes where the last one went,
// without the panel being opened at all. Nothing remembered is not a refusal — it is what the sheet
// is for — so the first save of a session is the sheet and the ones after it are this.
//
// A place the tree no longer has answers the same way: the collection was deleted, and the sheet is
// where that is found out. So does a line nothing has been done to since the last save.
async function saveShortcut() {
  // The panel is up, and the last row of it says what the key does there: the key is the panel's
  // while the panel is open, or the row would be advertising a key that saves somewhere else.
  if (saveOpen.value) {
    savePanel.value?.onSaveKey()
    return
  }
  const place = places.value.find((p) => p.id === collections.lastSaveTarget)
  if (!unsaved.value || !place) {
    saveOpen.value = true
    return
  }
  if (!(await collections.saveDraft(place.id, saveDefaultName.value))) return
  requests.kept()
  toast.show(t('collections.savedTo', { name: place.path.join(' / ') }))
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
  // Nothing came back: the reading could not be asked for, and that has been said already. The paste
  // was prevented, so the text would otherwise be lost — it goes into the field as plain text.
  if (!reading) {
    insertAtCaret(input, text)
    return
  }

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
            <template v-for="(piece, i) in urlParts" :key="i">
              <VarToken
                v-if="piece.tokenName"
                variant="plain"
                :name="piece.tokenName"
                :text="piece.text"
                :offset="piece.start"
                :env-id="store.environmentId"
              />
              <span v-else :class="piece.kind">{{ piece.text }}</span>
            </template>
          </div>
        </div>
      </div>

      <BarEnvironment :source="store" />

      <span v-if="canSave" ref="saveAnchor" class="save-anchor">
        <button
          class="bookmark-btn"
          :class="{ open: saveOpen }"
          :disabled="!store.url.trim()"
          :title="saveHint"
          @click="saveOpen = !saveOpen"
        >
          <Icon name="bookmark" :size="17" :stroke-width="1.7" />
          <span v-if="unsaved" class="unsaved-dot"></span>
        </button>
        <SaveRequestPopover
          ref="savePanel"
          :open="saveOpen"
          :url="store.url"
          :default-name="saveDefaultName"
          @update:open="saveOpen = $event"
          @saved="requests.kept()"
        />
      </span>

      <Button
        variant="primary"
        class="send-btn"
        :class="{ sending: store.loading }"
        :disabled="sendDisabled"
        :title="sendBlockedReason"
        @click="onSend"
      >
        <span v-if="store.loading" class="send-spinner"></span>
        <span>{{ sendLabel }}</span>
        <span class="send-hint">{{ sendKey }}</span>
      </Button>
    </div>

    <!-- What is holding the send back, standing under the address it is about rather than over the
         button it blocks. The sentence carries the names in place, so it is written on one line and
         gives way rather than wrapping. -->
    <div v-if="sendBlocked" class="var-error">
      <span class="var-error-dot"></span>
      <span v-if="scopeId === null" class="var-error-text" :title="sendBlockedReason">{{ t('request.missingVariable', missingVarNames.length) }} <span v-for="(n, i) in missingVarNames" :key="n" class="var-name mono">{{ n }}<span v-if="i < missingVarNames.length - 1">, </span></span> {{ t('request.missingNoEnv') }}</span>
      <span v-else class="var-error-text" :title="sendBlockedReason">{{ t('request.missingVariable', missingVarNames.length) }} <span v-for="(n, i) in missingVarNames" :key="n" class="var-name mono">{{ n }}<span v-if="i < missingVarNames.length - 1">, </span></span> {{ t('request.missingNotIn', missingVarNames.length) }} <b>{{ answersIn }}</b> {{ t('request.missingSendingBlocked') }}</span>

      <!-- One way out, as the drawing gives it. In an environment the user has closed the button is
           shut and its tooltip says what opens it — the sentence already names the environment, so
           the two read together. -->
      <button
        type="button"
        class="var-error-action"
        :disabled="scopeId === null || scopeReadonly"
        :title="createTitle"
        @click="createMissing"
      >
        {{ createLabel }}
      </button>
    </div>

    <BarSections :source="store" />
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* One line of the block's own rhythm: the address and the three controls that belong to it. */
.method-btn {
  @apply border-0 cursor-pointer outline-none;
  --wails-draggable: no-drag;
}

.url-text {
  /* The address keeps a floor of its own: it is the one thing in this row that cannot be guessed from
     anything else, so when the window is too narrow the buttons give way before it does. */
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

/* A query is read in two inks: what the parameters are called is quiet, what they say is not. */
.url-display .query-key {
  color: var(--text-tertiary);
}

.url-display .query-value {
  color: var(--accent);
}

.url-input:focus,
.url-input:focus-visible {
  border: 0;
  box-shadow: none;
}

/* ── The save button ───────────────────────────────────────────────────────────────────────────── */

.save-anchor {
  @apply relative flex-none;
}

/* The save button is the field's own height and nothing else: no frame, no fill, and the accent only
   when the pointer is on it — it is the quietest of the three things in this row. */
.bookmark-btn {
  @apply relative flex-none flex items-center justify-center cursor-pointer;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--text-secondary);
  --wails-draggable: no-drag;
}

/* The button wears the accent while its panel is open, hover or not: what it holds is on screen, and
   a control that looked the same open and closed would leave the panel attached to nothing. */
.bookmark-btn:hover:not(:disabled),
.bookmark-btn.open {
  background: var(--bg-hover);
  color: var(--accent);
}

.bookmark-btn:disabled {
  @apply opacity-50 cursor-default;
}

/* Something in the line no collection has: the ring is the bar's own material, so the dot reads as
   sitting on the button rather than beside it. */
.unsaved-dot {
  @apply absolute w-1.5 h-1.5 rounded-full bg-accent;
  top: 6px;
  right: 6px;
  box-shadow: 0 0 0 2px var(--glass-side);
}

/* ── The send button ───────────────────────────────────────────────────────────────────────────── */

/* A size of this bar's own rather than of the shared control: the field beside it is 34 tall, and
   the two are the same object seen twice. Named with the primitive's class as well, because a
   scoped override and the primitive's own scoped rule have the same specificity. */
.btn.send-btn {
  @apply flex-none;
  min-width: 104px;
  height: 34px;
  padding: 0 15px;
  border-radius: 7px;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
}

/* In flight the button is the way to stop it, and it says so in the colour of stopping: a red
   button beside a live counter is unmistakably the request, not the thing that starts it. */
.btn.send-btn.sending {
  background: var(--red);
  border-color: var(--red);
}

.send-spinner {
  @apply flex-none w-3.5 h-3.5 rounded-full;
  border: 2px solid color-mix(in srgb, #fff 35%, transparent);
  border-top-color: #fff;
  animation: spin 0.72s linear infinite;
}

.send-hint {
  @apply font-medium;
  font-family: var(--mono);
  font-size: 11px;
  opacity: 0.75;
}

/* ── What holds the send back ──────────────────────────────────────────────────────────────────── */

/* A notice inside the block rather than a strip across the window: it is about the address above it,
   and it stands where the sections stand — the shape of the thing it is blocking. */
/* The block stands under the address it is about and on the address's own edges: the bar is inset
   20px on both sides, and a warning that ran the full width of the window would not read as being
   about the line above it. */
.var-error {
  @apply flex items-center gap-[9px] mt-[7px] mx-5 py-[7px] px-2.5 rounded-[7px];
  background: var(--red-soft);
}

.var-error-dot {
  @apply flex-none w-1.5 h-1.5 rounded-full bg-red;
}

/* One sentence, and it gives way rather than wrapping: a long address in a tooltip is better than a
   second line that pushes the answer down. It is written in the colour of the thing that is wrong —
   the handoff paints the whole line, and a warning in the ordinary ink is a sentence the eye has to
   stop and read. `--red-text` rather than the fill: this is written words, and the fill is for the dot. */
.var-error-text {
  @apply flex-1 min-w-0 text-[12px];
  color: var(--red-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.var-error-text b {
  @apply font-semibold;
}

/* The names are values in a sentence about them, so they are drawn as values: the mono face the
   markup wears, and the sentence's own colour. A second red inside a red line would be a second
   thing to read, and what makes a name stand out here is the face and the weight, not the ink. */
.var-name {
  @apply font-semibold;
}

.var-error-action {
  @apply flex-none h-6 px-[9px] rounded-md cursor-pointer whitespace-nowrap text-[12px] font-medium;
  border: 1px solid var(--border-strong);
  background: var(--bg-inset);
  color: var(--text);
  font-family: inherit;
  --wails-draggable: no-drag;
}

.var-error-action:hover:not(:disabled) {
  background: var(--bg-hover);
}

.var-error-action:disabled {
  @apply opacity-50 cursor-default;
}

</style>
