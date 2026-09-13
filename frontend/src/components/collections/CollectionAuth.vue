<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCollectionsStore } from '../../stores/collections'
import { useMessages } from '../../i18n'
import { AuthType, type Auth } from '../../../bindings/json-inspector/internal/domain'

const { t } = useMessages()

// What a collection or a folder authorizes its requests with, set once for everything inside it. The
// three kinds are the design's: a level either sends a Bearer token, a Basic credential, or nothing —
// «Наследовать» is what a *request* below says, not what a level does.
const store = useCollectionsStore()

const KINDS: Auth['type'][] = [AuthType.AuthBearer, AuthType.AuthBasic, AuthType.AuthNone]
// The two wire schemes are named by their own names; only the app's own word is translated.
const LABELS: Record<string, string> = { bearer: 'Bearer Token', basic: 'Basic', none: t('request.auth.none') }

const NONE: Auth = { type: 'none' as Auth['type'], token: '' }

// The level the tab is about: the collection that is open, wherever it sits.
const levelId = computed(() => store.selectedId ?? '')
// What this level answers with *itself* — which is what the tab edits. A folder with nothing of its
// own is «Нет» rather than the token it inherits: showing the parent's here would make the next
// keystroke a copy of it, and inheriting is not owning.
const auth = computed<Auth>(() => store.selected?.auth ?? store.trail?.collection?.auth ?? NONE)
// What the levels above answer with, said in a line of its own. Only a node has levels above it: a
// collection is the top of its own tree.
const inherited = computed<Auth | null>(() => (store.selected ? store.inheritedAuth : null))
const inheritedLabel = computed(() => {
  const level = inherited.value
  if (!level) return ''
  const kind = LABELS[level.type] ?? level.type
  return level.token
    ? t('request.inheritedWithToken', { kind, token: level.token })
    : t('request.inherited', { kind })
})
const index = computed(() => Math.max(0, KINDS.indexOf(auth.value.type)))

const token = ref(auth.value.token)
// The token was typed over: what is in the field is not what Go holds, and the field is what the user
// is looking at — the same rule the description of a collection follows.
const dirty = ref(false)

watch([levelId, () => auth.value.type], () => {
  dirty.value = false
  token.value = auth.value.token
})

watch(
  () => auth.value.token,
  (value) => {
    if (!dirty.value) token.value = value
  }
)

// A kind is a click, so it is written at once; the token is typed, so it waits for the field to be
// left or for Enter — a write per keystroke would be a tree's worth of answers for one line.
async function choose(type: Auth['type']) {
  await store.saveAuth(levelId.value, { ...auth.value, type })
}

async function commitToken() {
  if (!dirty.value) return
  dirty.value = false
  if (token.value === auth.value.token) return
  await store.saveAuth(levelId.value, { ...auth.value, token: token.value })
}

// The field is bound one way — Go holds the value and the watcher above decides what is drawn there —
// so what was typed is taken off the event: a ref that stayed as it was would compare equal to the
// stored token and write nothing.
function onTokenInput(value: string) {
  token.value = value
  dirty.value = true
}

function onTokenKeydown(e: KeyboardEvent) {
  e.stopPropagation()
  if (e.key === 'Enter') {
    e.preventDefault()
    void commitToken()
  } else if (e.key === 'Escape') {
    e.preventDefault()
    dirty.value = false
    token.value = auth.value.token
  }
}

</script>

<template>
  <div class="auth">
    <div class="block">
      <span class="heading">{{ t('collections.authType') }}</span>
      <div class="segmented">
        <span class="seg-indicator" :style="{ transform: `translateX(calc(${index} * (100% + 2px)))` }" />
        <button
          v-for="kind in KINDS"
          :key="kind"
          class="seg"
          :class="{ active: auth.type === kind }"
          @click="choose(kind)"
        >
          {{ LABELS[kind] }}
        </button>
      </div>
    </div>

    <div v-if="inheritedLabel" class="inherited">{{ inheritedLabel }}</div>

    <label v-if="auth.type !== 'none'" class="block">
      <span class="label">{{ auth.type === 'basic' ? t('collections.loginPassword') : t('request.token') }}</span>
      <input
        :value="token"
        class="token mono"
        :placeholder="auth.type === 'basic' ? t('collections.loginPasswordPlaceholder') : t('request.token')"
        spellcheck="false"
        @input="onTokenInput(($event.target as HTMLInputElement).value)"
        @keydown="onTokenKeydown"
        @blur="commitToken"
      />
    </label>

    <div class="note">
      <svg
        class="note-icon"
        width="13"
        height="13"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v5M12 16h.01" stroke-linecap="round" />
      </svg>
      <span class="note-text">
        {{ t('collections.authNote') }}
      </span>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

/* The tab's own room: the panel is a form of one column, and the design gives it 460px of it. */
.auth {
  @apply flex flex-col gap-3 max-w-[460px] px-6 py-6;
}

.block {
  @apply flex flex-col gap-[5px];
}

.heading {
  @apply text-[13px] font-semibold;
}

.label {
  @apply text-[12px] font-medium;
}

/* What the level above answers with. It is a line rather than a value in the field: the field is
   this level's own answer, and the two are different things. */
.inherited {
  @apply text-[12px] text-text-tertiary;
}

.segmented {
  @apply relative flex gap-0.5 p-0.5 rounded-[7px] bg-bg-inset w-[320px];
}

.seg-indicator {
  @apply absolute top-0.5 left-0.5 h-[calc(100%-4px)] rounded-[5px] bg-bg-panel;
  width: calc((100% - 4px - 4px) / 3);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  transition: transform 0.18s ease;
}

.seg {
  @apply relative flex-1 text-center text-[12px] py-1.5 text-text-secondary cursor-pointer;
  border: none;
  background: transparent;
  font: inherit;
}

.seg.active {
  @apply text-text font-semibold;
}

.token {
  @apply w-full h-[34px] px-2.5 rounded-lg border border-border-strong bg-bg-panel
         text-[12.5px] text-text;
}

.token::placeholder {
  @apply text-text-tertiary;
}

.token:focus {
  @apply outline-none border-accent;
}

.note {
  @apply flex items-start gap-2 px-3 py-2.5 rounded-lg bg-accent-soft mt-1;
}

.note-icon {
  @apply flex-none mt-px text-accent;
}

.note-text {
  @apply text-[12px] leading-normal;
}
</style>
