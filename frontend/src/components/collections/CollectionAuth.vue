<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useCollectionsStore } from '../../stores/collections'
import { useEnvironmentsStore } from '../../stores/environments'
import { useAuthSchemes } from '../../composables/useAuthSchemes'
import { useMessages } from '../../i18n'
import AuthFields from '../request/AuthFields.vue'
import AuthTypeSwitch from '../request/AuthTypeSwitch.vue'
import { AuthType, type Auth } from '../../../bindings/json-inspector/internal/domain'

// What a collection or a folder authorizes its requests with, set once for everything inside it.
// «Наследовать» is offered on a folder and not on a collection at the top of its tree: a level that
// says "the one above decides" needs one above it to decide, and a request inside a folder that
// inherits goes on walking up to whatever did answer.
const store = useCollectionsStore()
const environments = useEnvironmentsStore()
const { t } = useMessages()
const { schemeOf } = useAuthSchemes()

const NONE: Auth = { type: AuthType.AuthNone, fields: {} }

// The level the sheet is about: the collection that is open, wherever it sits.
const levelId = computed(() => store.selectedId ?? '')
// What this level answers with *itself* — which is what the sheet edits. A folder with nothing of its
// own is «Нет» rather than the token it inherits: showing the parent's here would make the next
// keystroke a copy of it, and inheriting is not owning.
const stored = computed<Auth>(() => store.selected?.auth ?? store.trail?.collection?.auth ?? NONE)

// «Inherit» is offered where there is something to inherit from: a collection at the top of its tree
// has nothing above it, and a choice that means nothing there is a choice nobody should make.
const canInherit = computed(() => (store.trail?.ancestors.length ?? 0) > 0)

// The answers are the sheet's while it is open, and the footer is what hands them over: a scheme is a
// click and a token is typing, and neither is written down until Save says so.
const draft = ref<Auth>(stored.value)

watch([levelId, () => stored.value.type, () => stored.value.fields], () => {
  draft.value = { type: stored.value.type, fields: { ...(stored.value.fields ?? {}) } }
})

const scheme = computed(() => schemeOf(draft.value.type))

// A scheme is a click, and the answers already given stay where they are: each scheme keeps its own,
// so a level that was Bearer, looked at Basic and went back to Bearer still has its token.
function choose(type: Auth['type']) {
  if (type === draft.value.type) return
  draft.value = { type, fields: draft.value.fields }
}

function onField(key: string, value: string) {
  draft.value = { ...draft.value, fields: { ...draft.value.fields, [key]: value } }
}

async function commit() {
  await store.saveAuth(levelId.value, draft.value)
}

function discard() {
  draft.value = { type: stored.value.type, fields: { ...(stored.value.fields ?? {}) } }
}

// The token a scheme fetches is not an answer anybody types: where one is asked of a server, the
// sheet says where it will be kept rather than pretending a field is what gets it.
function openEnvironment() {
  environments.openSheet({ envId: environments.activeId ?? null, varName: '' })
}

defineExpose({ commit, discard })
</script>

<template>
  <div class="auth">
    <div class="block">
      <span class="heading">{{ t('collections.authType') }}</span>
      <AuthTypeSwitch
        :can-inherit="canInherit"
        :value="draft.type"
        scale="roomy"
        @select="choose"
      />
    </div>

    <div class="block">
      <AuthFields
        v-if="scheme?.fields?.length"
        :scheme="scheme"
        :auth="draft"
        scale="roomy"
        @update="onField"
      />
      <p class="note">{{ scheme?.note ? t(scheme.note) : t('collections.authNote') }}</p>
    </div>

  </div>
</template>

<style scoped>
@reference "../../style.css";

.auth {
  @apply flex flex-col gap-3.5;
}

.block {
  @apply flex flex-col gap-[7px];
}

/* The sheet's own label: small caps, like the blocks of the page it was opened from. */
.heading {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

/* The sentence about the scheme stands under its fields, which is where the drawing keeps it: what
   the fields are for is read after they are, not before. */
.hint,
.note {
  @apply text-[12.5px] text-text-tertiary;
  line-height: 1.5;
}

/* The way out to where a secret actually lives: the sheet keeps none, and the link says so by
   pointing at the one that does. */
.link {
  @apply self-start h-[30px] px-2 rounded-[7px] border-0 bg-transparent
         text-[13px] font-medium text-accent cursor-pointer;
  font-family: inherit;
}

.link:hover {
  @apply bg-accent-soft;
}
</style>
