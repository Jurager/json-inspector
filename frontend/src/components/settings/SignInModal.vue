<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Browser } from '@wailsio/runtime'
import Icon from '../ui/Icon.vue'
import { useAccount } from '../../composables/useAccount'
import { useMessages } from '../../i18n'

// The sign-in, in the drawing's 400px modal: a code the server hands out, a browser that opens on the
// page where it is confirmed, and a wait. There is no password anywhere, and the modal is the whole of
// what the app asks of the person.
//
// The steps are the drawing's three: start, waiting, done — plus the refusal, which the drawing does
// not draw and a person needs.
const props = defineProps<{ server: string }>()

const emit = defineEmits<{
  (e: 'begin', server: string): void
  (e: 'cancel'): void
  (e: 'close'): void
}>()

const { t } = useMessages()
// The dialog asks; the window answers. Starting a sign-in and giving one up both travel as events —
// the window is what holds the account's dialogs, and a component that called them itself would be
// the second place that decides what a click means.
const { state, failure, setServer } = useAccount()

// What the modal is showing. It starts where the person is, and every step after that is decided by
// what Go says: the code is out, the code was confirmed, the code came to nothing.
const step = ref<'start' | 'waiting' | 'done' | 'refused'>('start')
const customOpen = ref(false)
const address = ref(props.server)
const applying = ref(false)

const challenge = computed(() => state.value?.challenge ?? null)
const code = computed(() => challenge.value?.userCode ?? '')

// The wait is drawn from the state and not from the click: a sign-in that stopped — cancelled,
// ended on the other side, or never started — puts the dialog back to where a person chooses what
// to do, rather than leaving a spinner turning over nothing.
watch(
  () => state.value?.waiting,
  (waiting) => {
    step.value = waiting ? 'waiting' : 'start'
  }
)

watch(
  () => state.value?.signedIn,
  (signedIn) => {
    if (signedIn) step.value = 'done'
  }
)

watch(failure, (said) => {
  // A refusal while the app is not waiting is the answer to what was asked: the sign-in is over, and
  // it is over unhappy.
  if (said && !state.value?.waiting) step.value = 'refused'
})

function start() {
  step.value = 'start'
  emit('begin', address.value || props.server)
}

// The address is applied on its own: a person who typed it wants it kept, and the sign-in that
// follows is their next click.
async function apply() {
  applying.value = true
  try {
    await setServer(address.value)
  } finally {
    applying.value = false
  }
}

function openAgain() {
  if (challenge.value) void Browser.OpenURL(challenge.value.url)
}

function giveUp() {
  emit('cancel')
}
</script>

<template>
  <!-- The dialog is dismissed by its own buttons and by nothing else: a click that lands beside the
       card is a click that missed, and it must not throw away a code somebody is reading. -->
  <div class="modal-overlay">
    <div class="modal">
      <header class="modal-head">
        <span class="modal-title">{{ t('account.dialog.title') }}</span>
        <button class="modal-close" :title="t('common.close')" @click="emit('close')">
          <Icon name="xmark" :size="14" :stroke-width="2.2" />
        </button>
      </header>

      <div v-if="step === 'start'" class="modal-body start">
        <span class="mark">
          <Icon name="file" :size="24" :stroke-width="1.7" />
        </span>
        <div class="titles">
          <span class="title">{{ t('account.dialog.startTitle') }}</span>
          <span class="text">{{ t('account.dialog.startText') }}</span>
        </div>

        <button class="wide" @click="start">
          <Icon name="window" :size="15" :stroke-width="1.9" />
          {{ t('account.dialog.browser') }}
        </button>

        <div class="links">
          <button class="link" @click="emit('close')">{{ t('account.dialog.without') }}</button>
          <span class="dot">·</span>
          <button class="link" @click="customOpen = !customOpen">
            {{ t('account.dialog.custom') }}
            <Icon name="chevron-down" :size="10" :stroke-width="2.6" />
          </button>
        </div>

        <div v-if="customOpen" class="server">
          <span class="server-label">{{ t('account.dialog.serverLabel') }}</span>
          <div class="server-row">
            <input v-model="address" class="server-field" spellcheck="false" />
            <button class="server-apply" :disabled="applying" @click="apply">
              {{ t('account.dialog.apply') }}
            </button>
          </div>
          <span class="server-hint">{{ t('account.dialog.serverHint') }}</span>
        </div>

        <p v-if="failure" class="failure">{{ failure }}</p>
      </div>

      <div v-else-if="step === 'waiting'" class="modal-body waiting">
        <span class="spinner"></span>
        <div class="titles">
          <span class="wait-title">{{ t('account.dialog.waitingTitle') }}</span>
          <span class="text">{{ t('account.dialog.waitingText') }}</span>
        </div>
        <!-- The code is here as well as in the browser: a person whose browser did not open can type it
             into the page themselves, and that is the whole reason the server prints a short one. -->
        <div v-if="code" class="device-code">
          <span class="device-code-label">{{ t('account.dialog.codeLabel') }}</span>
          <span class="device-code-value">{{ code }}</span>
        </div>
        <div class="links">
          <button class="link accent" @click="openAgain">{{ t('account.dialog.openAgain') }}</button>
          <button class="link" @click="giveUp">{{ t('account.dialog.cancel') }}</button>
        </div>
      </div>

      <div v-else class="modal-body start">
        <span class="mark" :class="{ bad: step === 'refused' }">
          <Icon :name="step === 'done' ? 'check' : 'xmark'" :size="24" :stroke-width="1.9" />
        </span>
        <div class="titles">
          <span class="title">
            {{ step === 'done' ? t('account.dialog.doneTitle') : t('account.dialog.refusedTitle') }}
          </span>
          <span class="text">{{ step === 'done' ? t('account.dialog.doneText') : failure }}</span>
        </div>
        <button v-if="step === 'refused'" class="wide" @click="start">
          {{ t('account.dialog.retry') }}
        </button>
        <button v-else class="wide" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The drawing's modal: 400px, a 40px head with the name in it, and a body that is one column centred
   on what it holds. It is narrower than a sheet because it asks one thing. */
.modal-overlay {
  @apply fixed inset-0 z-1600 flex items-center justify-center p-7;
  background: rgba(0, 0, 0, 0.22);
}

.modal {
  @apply flex flex-col w-[400px] max-w-[92vw] rounded-[12px] overflow-hidden;
  background: var(--glass-sheet);
  backdrop-filter: var(--blur-sheet);
  box-shadow: var(--glass-sheet-shadow), 0 0 0 1px var(--glass-overlay-border);
}

.modal-head {
  @apply relative flex-none h-[40px] flex items-center justify-center border-b border-border;
}

.modal-title {
  @apply text-[13px] font-semibold text-text-secondary;
}

.modal-close {
  @apply absolute right-2 inline-flex items-center justify-center w-[26px] h-[26px]
         rounded-md border-0 bg-transparent text-text-tertiary cursor-pointer;
  --wails-draggable: no-drag;
}

.modal-close:hover {
  @apply bg-bg-active text-text;
}

.modal-body {
  @apply flex flex-col items-center gap-5 text-center;
}

.start {
  padding: 32px 30px 26px;
}

.waiting {
  padding: 38px 30px 32px;
}

.mark {
  @apply inline-flex items-center justify-center w-12 h-12 rounded-xl bg-accent text-accent-text;
}

.mark.bad {
  @apply bg-bg-inset text-text-secondary;
}

.titles {
  @apply flex flex-col items-center gap-[7px];
}

.title {
  @apply text-[16px] font-semibold tracking-[-0.01em];
}

.wait-title {
  @apply text-[14px] font-semibold;
}

.text {
  @apply text-[13px] text-text-secondary max-w-[290px];
  line-height: 1.5;
}

.wide {
  @apply w-full h-9 inline-flex items-center justify-center gap-[9px] rounded-lg border-0
         bg-accent text-accent-text text-[13px] font-semibold cursor-pointer;
  font-family: inherit;
}

/* The primary button's own hover, from the button component: the accent brightens rather than
   changing colour, which is what keeps one accent across the app. */
.wide:hover {
  @apply brightness-110;
}

.links {
  @apply flex items-center gap-2.5 text-[13px];
}

.dot {
  @apply text-border;
}

.link {
  @apply inline-flex items-center gap-[5px] border-0 bg-transparent p-0.5 text-[13px]
         text-text-tertiary cursor-pointer;
  font-family: inherit;
}

.link:hover {
  @apply text-text;
}

.link.accent {
  @apply text-[13px] font-medium text-accent;
}

.link.accent:hover {
  @apply underline;
}

.server {
  @apply w-full flex flex-col gap-[7px] border border-border rounded-[12px] bg-bg-inset
         px-3 py-[11px] text-left;
}

.server-label {
  @apply text-[11px] font-semibold tracking-[0.07em] uppercase text-text-tertiary;
}

.server-row {
  @apply flex gap-[7px];
}

.server-field {
  @apply flex-1 min-w-0 h-[30px] px-2.5 border border-border-strong rounded-[7px] bg-bg-panel
         text-[12px] text-text-secondary;
  font-family: ui-monospace, 'SF Mono', 'Cascadia Code', Menlo, monospace;
}

.server-apply {
  @apply flex-none h-[30px] px-[11px] border border-border-strong rounded-[7px] bg-bg-panel
         text-text text-[13px] cursor-pointer;
  font-family: inherit;
}

.server-apply:hover {
  @apply bg-bg-hover;
}

.server-hint {
  @apply text-[11px] text-text-tertiary;
  line-height: 1.45;
}

/* The wait, as the updated drawing has it: the sentence is narrower than the first step's, and the
   two links are a size up and further apart — this screen holds one thing to read and one to press. */
.waiting .text {
  max-width: 260px;
}

.waiting .links {
  gap: 14px;
}

.waiting .link {
  @apply text-[13px];
}

/* The code the person checks against the program's screen. The drawing sets it in the window's own
   face and not in a mono one: it is read off one screen and typed into another, and what makes the
   pairs tell apart at that distance is the weight and the tabular digits, not the typeface. */
.device-code {
  @apply flex flex-col items-center;
  gap: 6px;
}

.device-code-label {
  @apply text-[11px] font-semibold tracking-[0.09em] uppercase text-text-tertiary;
}

.device-code-value {
  @apply text-[22px] font-bold tracking-[0.05em];
  font-variant-numeric: tabular-nums;
}

.spinner {
  @apply inline-block w-11 h-11 rounded-full;
  border: 2.5px solid var(--accent-soft);
  border-top-color: var(--accent);
  animation: spin 0.9s linear infinite;
}

.failure {
  @apply m-0 text-[13px] text-red;
}
</style>
