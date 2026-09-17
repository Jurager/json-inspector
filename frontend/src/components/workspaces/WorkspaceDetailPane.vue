<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import ColorSwatches from '../ui/ColorSwatches.vue'
import Segmented from '../environments/Segmented.vue'
import Tooltip from '../ui/tooltip/Tooltip.vue'
import Icon from '../ui/Icon.vue'
import { WORKSPACE_COLORS, WORKSPACE_TINTS } from './palette'
import { useWorkspacesStore } from '../../stores/workspaces'
import { useWorkspaceCounts } from '../../composables/useWorkspaceCounts'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'

// One workspace: what it is called, what colour it wears, what it is meant to be, who is in it, and
// what it holds — and, at the foot, the one thing that takes it all away again.
//
// Type and Members are drawn and not offered: the model has told a personal space from a team one from
// the start, and everything a team adds — invitations, roles, a place to keep them — is not in this
// build. They are here because the drawing has them and because what they will say is worth seeing.
const emit = defineEmits<{ (e: 'danger'): void }>()

const store = useWorkspacesStore()
const { contentsOf } = useWorkspaceCounts()
const { notice, setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

const workspace = computed(() => store.edited)

const colorLabels = computed<Record<string, string>>(() => ({
  blue: t('workspaces.colorBlue'),
  purple: t('workspaces.colorPurple'),
  green: t('workspaces.colorGreen'),
  orange: t('workspaces.colorOrange'),
  grey: t('workspaces.colorGrey'),
}))

const kinds = computed(() => [
  { value: 'personal', label: t('workspaces.kindPersonal') },
  { value: 'team', label: t('workspaces.kindTeam'), soon: true },
])

// What the segment shows: the space's own kind, which is the personal one for everything this build
// can make. It is a reading, not a choice — nothing here writes a kind.
const kind = computed(() => workspace.value?.kind ?? 'personal')

const name = ref('')
const invalid = ref(false)
const nameInput = ref<HTMLInputElement | null>(null)

// The field follows what the pane is showing: moving between rows is not an edit, and a name that was
// refused leaves nothing behind once another space is on screen.
watch([() => workspace.value?.id, () => workspace.value?.name], syncName, { immediate: true })

function syncName() {
  name.value = workspace.value?.name ?? ''
  invalid.value = false
}

function refuse(message: string) {
  invalid.value = true
  setNotice(message)
  nextTick(() => nameInput.value?.select())
}

// Renaming happens here: Enter and leaving the field both commit, Escape puts the stored name back,
// and a refused one keeps the field — with what was typed still in it — so it can be fixed.
async function commit() {
  const target = workspace.value
  if (!target) return
  const wanted = name.value.trim()

  if (wanted === target.name) {
    invalid.value = false
    clearNotice()
    return
  }
  if (!wanted) {
    refuse(t('environments.nameEmpty'))
    return
  }
  if (store.list.some((w) => w.id !== target.id && w.name === wanted)) {
    refuse(t('environments.nameTaken'))
    return
  }

  invalid.value = false
  clearNotice()
  try {
    await store.update(target.id, { name: wanted })
  } catch (error) {
    refuse(describeFailure(error))
  }
}

function revert() {
  name.value = workspace.value?.name ?? ''
  invalid.value = false
  clearNotice()
}

// A colour is written the moment it is picked, like an environment's: the window wears it at once, and
// there is no Save to forget.
function pickColor(color: string) {
  const target = workspace.value
  if (!target) return
  void store.update(target.id, { color })
}

const contents = computed(() =>
  workspace.value ? contentsOf(workspace.value) : []
)

// The one space left cannot go: every other answer the app gives leans on there being somewhere to
// keep things. The button stays drawn and is shut, and its tooltip is what says why.
const last = computed(() => store.list.length <= 1)

// Escape belongs to the innermost thing on screen: the field being edited first, and the window after
// it. Nothing in this pane holds a cell, so it answers for the field alone.
function cancelTop(): boolean {
  if (!invalid.value) return false
  revert()
  return true
}

defineExpose({ cancelTop })
</script>

<template>
  <div class="pane">
    <div v-if="workspace" class="pane-body">
      <div class="pair">
        <label class="field">
          <span class="field-label">{{ t('workspaces.nameLabel') }}</span>
          <input
            ref="nameInput"
            v-model="name"
            class="input"
            :class="{ invalid }"
            maxlength="40"
            spellcheck="false"
            @keydown.enter="commit"
            @keydown.escape.prevent="revert"
            @blur="commit"
          />
        </label>

        <div class="field tight">
          <span class="field-label">{{ t('workspaces.colorLabel') }}</span>
          <ColorSwatches
            :value="workspace.color"
            :colors="WORKSPACE_COLORS"
            :tints="WORKSPACE_TINTS"
            :labels="colorLabels"
            ground="var(--glass-sheet)"
            @pick="pickColor"
          />
        </div>
      </div>

      <div class="field">
        <span class="field-label">{{ t('workspaces.kindLabel') }}</span>
        <Segmented
          class="kinds"
          :options="kinds"
          :value="kind"
          :soon-hint="t('workspaces.kindTeamSoon')"
        />
      </div>

      <div class="field">
        <span class="field-label">{{ t('workspaces.membersLabel', { count: 1 }) }}</span>
        <div class="card">
          <div class="member">
            <span class="member-avatar">
              <Icon name="user" :size="12" />
            </span>
            <span class="member-name">{{ t('workspaces.you') }}</span>
            <span class="member-role">{{ t('workspaces.ownerRole') }}</span>
          </div>
          <div class="member muted">
            <span class="member-avatar quiet">
              <Icon name="plus" :size="12" />
            </span>
            <span class="member-name">{{ t('workspaces.inviteSoon') }}</span>
          </div>
        </div>
      </div>

      <div class="field">
        <span class="field-label">{{ t('workspaces.contentsLabel') }}</span>
        <div class="contents">
          <div v-for="entry in contents" :key="entry.label" class="count-card">
            <span class="count-value">{{ entry.value }}</span>
            <span class="count-label">{{ entry.label }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="foot">
      <span class="danger-note">{{ t('workspaces.dangerNote') }}</span>
      <!-- A shut button fires no pointer events at all, so the tooltip hangs on the box around it:
           the drawing is the same, and the sentence is what explains why it is shut. -->
      <Tooltip v-if="last">
        <template #trigger>
          <span class="danger disabled">{{ t('workspaces.removeAction') }}</span>
        </template>
        {{ t('workspaces.lastHint') }}
      </Tooltip>
      <button v-else type="button" class="danger" @click="emit('danger')">
        {{ t('workspaces.removeAction') }}
      </button>
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

.input {
  @apply w-full h-[34px] px-[11px] rounded-lg text-[13.5px] text-text outline-none;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

/* A name that was refused: the frame says which field it is about, and the notice beside the buttons
   says what is wrong with it. */
.input.invalid,
.input.invalid:focus {
  border-color: color-mix(in srgb, var(--red) 45%, transparent);
}

.input.invalid:focus {
  box-shadow: 0 0 0 3px var(--red-soft);
}

.kinds {
  max-width: 260px;
}

.card {
  @apply flex flex-col gap-px rounded-[10px] border border-border p-1;
}

.member {
  @apply flex items-center gap-2 py-1.5 px-1.5;
}

.member-avatar {
  @apply flex-none inline-flex items-center justify-center w-[22px] h-[22px] rounded-full
         bg-accent-soft text-accent;
}

.member-avatar.quiet {
  @apply bg-bg-hover text-text-tertiary;
}

.member-name {
  @apply flex-1 min-w-0 text-[13px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.member.muted .member-name {
  @apply text-text-tertiary;
}

.member-role {
  @apply flex-none text-[10.5px] text-text-tertiary;
}

.contents {
  @apply grid gap-2.5;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

/* A number and the thing it counts, in one card: the drawing gives the figure the weight and lets the
   word agree with it — which is why the label is a counted word and not a fixed one. */
.count-card {
  @apply border border-border rounded-[10px] p-3 flex flex-col gap-1;
}

.count-value {
  @apply text-[18px] font-semibold;
  letter-spacing: -0.01em;
}

.count-label {
  @apply text-[12.5px] text-text-tertiary;
}

.foot {
  @apply flex-none flex items-center h-12 px-5 gap-3;
  border-top: 1px solid var(--border);
}

.danger-note {
  @apply flex-1 min-w-0 text-[12.5px] text-text-tertiary;
}

.danger {
  @apply flex-none h-[30px] px-3 rounded-[7px] cursor-pointer text-[12.5px] font-medium;
  font-family: inherit;
  color: var(--red-text);
  background: transparent;
  border: 1px solid var(--red-soft);
}

.danger:hover:not(.disabled) {
  @apply bg-red-soft;
}

/* The shut one is drawn as the same button and is not one: nothing happens, and the tooltip over it
   is the whole of what it says. */
.danger.disabled {
  @apply inline-flex items-center opacity-50 cursor-default;
}
</style>
