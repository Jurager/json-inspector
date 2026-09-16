<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCollectionsStore } from '../../stores/collections'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import { useMessages } from '../../i18n'
import AuthFields from '../request/AuthFields.vue'
import AuthTypeSwitch from '../request/AuthTypeSwitch.vue'
import { AuthType, type Auth } from '../../../bindings/json-inspector/internal/domain'

// What a collection or a folder authorizes its requests with, set once for everything inside it.
// «Наследовать» is not offered here: that is what a *request* below says about where it takes its
// authorization from, and a level is not a request.
const store = useCollectionsStore()
const { t } = useMessages()
const { schemeOf } = useAuthSchemes()

const NONE: Auth = { type: AuthType.AuthNone, fields: {} }

// The level the tab is about: the collection that is open, wherever it sits.
const levelId = computed(() => store.selectedId ?? '')
// What this level answers with *itself* — which is what the tab edits. A folder with nothing of its
// own is «Нет» rather than the token it inherits: showing the parent's here would make the next
// keystroke a copy of it, and inheriting is not owning.
const stored = computed<Auth>(() => store.selected?.auth ?? store.trail?.collection?.auth ?? NONE)

// What the levels above answer with, said in a line of its own. Only a node has levels above it: a
// collection is the top of its own tree.
const inherited = computed<Auth | null>(() => (store.selected ? store.inheritedAuth : null))
const inheritedLabel = computed(() => {
  const level = inherited.value
  if (!level) return ''
  const above = schemeOf(level.type)
  return t('request.inherited', { kind: above ? t(above.label) : level.type })
})

// The answers are the field's while it is being typed in. A tree's authorization saves the whole
// tree, and a write per keystroke would be a tree's worth of answers for one line — so the tab holds
// them and hands them over when the field is left.
const draft = ref<Auth>(stored.value)
const typing = ref(false)

watch([levelId, () => stored.value.type], () => {
  typing.value = false
  draft.value = stored.value
})

watch(
  () => stored.value.fields,
  (fields) => {
    if (!typing.value) draft.value = { type: stored.value.type, fields: fields ?? {} }
  }
)

const scheme = computed(() => schemeOf(draft.value.type))

// A scheme is a click, so it is written at once; the answers are typed, so they wait for the field
// to be left or for Enter.
//
// The answers already given stay where they are: each scheme keeps its own, so a level that was
// Bearer, looked at Basic and went back to Bearer still has its token. Clicking the scheme already
// in use changes nothing at all.
async function choose(type: Auth['type']) {
  if (type === draft.value.type) return
  typing.value = false
  draft.value = { type, fields: draft.value.fields }
  await store.saveAuth(levelId.value, draft.value)
}

function onField(key: string, value: string) {
  typing.value = true
  draft.value = { ...draft.value, fields: { ...draft.value.fields, [key]: value } }
}

async function commit() {
  if (!typing.value) return
  typing.value = false
  await store.saveAuth(levelId.value, draft.value)
}
</script>

<template>
  <div class="auth">
    <div class="block">
      <span class="heading">{{ t('collections.authType') }}</span>
      <AuthTypeSwitch :can-inherit="false" :value="draft.type" scale="roomy" @select="choose" />
    </div>

    <div v-if="inheritedLabel" class="inherited">{{ inheritedLabel }}</div>

    <p v-if="scheme?.note" class="hint">{{ t(scheme.note) }}</p>
    <AuthFields
      v-if="scheme?.fields?.length"
      :scheme="scheme"
      :auth="draft"
      scale="roomy"
      @update="onField"
      @commit="commit"
    />

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
  @apply flex flex-col gap-[6px];
}

.heading {
  @apply text-[13px] font-semibold;
}

/* What the level above answers with. It is a line rather than a value in the field: the field is
   this level's own answer, and the two are different things. */
.inherited {
  @apply text-[12px] text-text-tertiary;
}

.hint {
  @apply text-[12px] text-text-tertiary;
  line-height: 1.5;
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
