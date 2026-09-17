<script setup lang="ts">
import { computed, ref } from 'vue'
import ColorSwatches from '../ui/ColorSwatches.vue'
import Segment from '../ui/Segment.vue'
import { WORKSPACE_COLORS, WORKSPACE_TINTS } from './palette'
import { useWorkspacesStore } from '../../stores/workspaces'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'

// The form that makes a space: a name, a colour, and what the space is meant to be. The Type is drawn
// and not offered — the model has told the two apart from the start, and the form that makes a team is
// the one that will also have to invite its first members, which this build has nowhere to put.
const emit = defineEmits<{ (e: 'cancel'): void; (e: 'created'): void }>()

const store = useWorkspacesStore()
const { notice, setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

const colorLabels = computed<Record<string, string>>(() => ({
  blue: t('workspaces.colorBlue'),
  purple: t('workspaces.colorPurple'),
  green: t('workspaces.colorGreen'),
  orange: t('workspaces.colorOrange'),
  grey: t('workspaces.colorGrey'),
}))

// Personal is what a space made here is; Team is drawn beside it so the answer the model already gives
// is visible, and `soon` is what keeps it from being chosen.
const kinds = computed(() => [
  { value: 'personal', label: t('workspaces.kindPersonal') },
  { value: 'team', label: t('workspaces.kindTeam'), soon: true },
])

const name = ref('')
const color = ref<string>(WORKSPACE_COLORS[0])

async function create() {
  const wanted = name.value.trim()
  if (!wanted) {
    setNotice(t('environments.nameEmpty'))
    return
  }
  if (store.list.some((w) => w.name === wanted)) {
    setNotice(t('environments.nameTaken'))
    return
  }

  try {
    await store.create({ name: wanted, color: color.value })
  } catch (error) {
    setNotice(describeFailure(error))
    return
  }
  clearNotice()
  emit('created')
}

function cancel() {
  clearNotice()
  emit('cancel')
}
</script>

<template>
  <div class="pane">
    <div class="pane-body">
      <span class="pane-title">{{ t('workspaces.newTitle') }}</span>

      <div class="pair">
        <label class="field">
          <span class="field-label">{{ t('workspaces.nameLabel') }}</span>
          <input
            v-model="name"
            class="input"
            maxlength="40"
            spellcheck="false"
            :placeholder="t('workspaces.namePlaceholder')"
            @keydown.enter="create"
          />
        </label>

        <div class="field tight">
          <span class="field-label">{{ t('workspaces.colorLabel') }}</span>
          <ColorSwatches
            :value="color"
            :colors="WORKSPACE_COLORS"
            :tints="WORKSPACE_TINTS"
            :labels="colorLabels"
            @pick="color = $event"
          />
        </div>
      </div>

      <div class="field">
        <span class="field-label">{{ t('workspaces.kindLabel') }}</span>
        <Segment
          class="kinds"
          :options="kinds"
          :value="'personal'"
          :soon-hint="t('workspaces.kindTeamSoon')"
        />
      </div>
    </div>

    <div class="foot">
      <span class="notice" :class="{ bad: !!notice }">{{ notice }}</span>
      <button type="button" class="pane-cancel" @click="cancel">{{ t('common.cancel') }}</button>
      <button type="button" class="pane-action" @click="create">{{ t('workspaces.createAction') }}</button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.pane {
  @apply flex-1 min-w-0 flex flex-col min-h-0;
}

.pane-body {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col gap-[18px];
  padding: 18px 20px;
}

.pane-title {
  @apply text-[15px] font-semibold;
}

.pair {
  @apply flex gap-3.5 items-end;
}

.field {
  @apply flex flex-col gap-1.5 min-w-0;
}

.pair > .field {
  @apply flex-1;
}

.pair > .field.tight {
  @apply flex-none;
}

.field-label {
  @apply text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

/* The one field a person is meant to type into first is drawn ready for it: the accent's own frame
   and the ring around it, which is what the drawing shows instead of a caret nobody can see. */
.input {
  @apply w-full h-[34px] px-[11px] rounded-lg text-[13.5px] text-text outline-none;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.kinds {
  max-width: 260px;
}

.foot {
  @apply flex-none flex items-center h-12 px-5 gap-2.5;
  border-top: 1px solid var(--border);
}

.notice {
  @apply flex-1 min-w-0 text-[12.5px] text-text-tertiary;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notice.bad {
  @apply text-red;
}

.pane-cancel {
  @apply flex-none h-[34px] px-3.5 rounded-lg cursor-pointer text-[13.5px] font-medium text-text;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.pane-cancel:hover {
  @apply bg-bg-hover;
}

.pane-action {
  @apply flex-none h-[34px] px-4 rounded-lg border-0 cursor-pointer text-accent-text
         text-[13.5px] font-semibold;
  font-family: inherit;
  background: var(--accent);
}

.pane-action:hover {
  @apply brightness-110;
}
</style>
