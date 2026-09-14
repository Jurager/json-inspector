<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from '../ui/dialog/Dialog.vue'
import { Button } from '../ui/button'
import Icon from '../ui/Icon.vue'
import Input from '../ui/input/Input.vue'
import Tooltip from '../ui/tooltip/Tooltip.vue'
import WorkspaceAvatar from './WorkspaceAvatar.vue'
import DeleteWorkspaceDialog from './DeleteWorkspaceDialog.vue'
import { WORKSPACE_COLORS, tintOf } from './palette'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { describeFailure, useMessages } from '../../i18n'

const props = defineProps<{ open: boolean }>()

const emit = defineEmits<{ (e: 'close'): void }>()

const store = useWorkspacesStore()

const { t } = useMessages()

const name = ref('')
const color = ref('')
const failure = ref('')
const confirming = ref(false)

const workspace = computed(() => store.edited)

// The card opens on what Go holds, and an edit made here is not a copy of it until it is saved:
// closing without saving puts the fields back to what the workspace is.
watch(
  () => [props.open, workspace.value?.id, workspace.value?.name, workspace.value?.color],
  () => {
    // A card that opens starts from the workspace and from nothing else: a colour left over from the
    // last time it was open is not a colour anybody chose just now.
    store.stopTrying()
    if (!props.open || !workspace.value) return
    name.value = workspace.value.name
    color.value = workspace.value.color
    failure.value = ''
    confirming.value = false
  },
  { immediate: true }
)

async function save() {
  if (!workspace.value) return
  try {
    await store.update(workspace.value.id, { name: name.value, color: color.value })
  } catch (err) {
    failure.value = describeFailure(err)
    return
  }
  // The colour is the workspace's now, so there is nothing left to try on: the tint follows the
  // saved value from here.
  store.stopTrying()
  emit('close')
}

// What choosing a colour does before it is saved: the window wears it. The window is the only place
// the choice can be judged — a swatch says what the colour is, not what a room of it looks like — and
// a card closed without saving takes it back off.
function tryColor(tint: string) {
  color.value = color.value === tint ? '' : tint
  if (workspace.value?.id === store.activeId) store.tryColor(color.value)
}

function close() {
  store.stopTrying()
  emit('close')
}

async function remove() {
  if (!workspace.value) return
  await store.remove(workspace.value.id)
  confirming.value = false
  close()
}
</script>

<template>
  <Dialog :open="open" class="w-[380px] p-4.5" @escape-key-down.prevent @update:open="close">
    <div class="card-title">{{ t('workspaces.settingsTitle') }}</div>

    <template v-if="workspace">
      <div class="field">
        <span class="label">{{ t('workspaces.nameLabel') }}</span>
        <Input v-model="name" size="md" @keydown.enter="save" />
      </div>

      <div class="field">
        <span class="label">{{ t('workspaces.membersLabel', { count: 1 }) }}</span>
        <div class="members">
          <!-- The one person the app knows is the one at this keyboard: there is no account yet, so
               the row has no address to show and the circle has a figure instead of initials. -->
          <div class="member">
            <span class="member-avatar"><Icon name="user" :size="12" /></span>
            <span class="member-name">{{ t('workspaces.you') }}</span>
            <span class="member-role">{{ t('workspaces.ownerRole') }}</span>
          </div>
          <div class="member">
            <span class="member-avatar quiet"><Icon name="plus" :size="12" /></span>
            <span class="member-name muted">{{ t('workspaces.inviteSoon') }}</span>
          </div>
        </div>
      </div>

      <div class="field">
        <span class="label">{{ t('workspaces.colorLabel') }}</span>
        <div class="swatches">
          <button
            v-for="tint in WORKSPACE_COLORS"
            :key="tint"
            type="button"
            class="swatch"
            :class="{ chosen: color === tint }"
            :style="{ background: tintOf(tint), '--tint': tintOf(tint) }"
            @click="tryColor(tint)"
          ></button>
        </div>
      </div>

      <p v-if="failure" class="failure">{{ failure }}</p>

      <div class="foot">
        <!-- The workspace the app is born with stays: it is where every fallback points, and a
             database without it has no space to show. The rule is Go's; this only draws it. -->
        <Tooltip v-if="workspace.personal">
          <template #trigger>
            <span class="remove disabled">{{ t('workspaces.removeAction') }}</span>
          </template>
          {{ t('workspaces.personalHint') }}
        </Tooltip>
        <button v-else class="remove" @click="confirming = true">
          {{ t('workspaces.removeAction') }}
        </button>

        <Button variant="primary" @click="save">{{ t('common.save') }}</Button>
      </div>
    </template>

    <DeleteWorkspaceDialog
      :open="confirming"
      :name="workspace ? workspaceName(workspace) : ''"
      @cancel="confirming = false"
      @confirm="remove"
    />
  </Dialog>
</template>

<style scoped>
@reference "../../style.css";

.card-title {
  @apply text-[14px] font-semibold mb-3.5;
}

.field {
  @apply flex flex-col gap-[5px] mb-3.5;
}

.label {
  @apply text-[12px] font-medium;
}

.members {
  @apply flex flex-col gap-px rounded-lg border border-border p-1;
}

.member {
  @apply flex items-center gap-2 py-1.5 px-1.5;
}

.member-avatar {
  @apply flex-none w-[22px] h-[22px] rounded-full inline-flex items-center justify-center bg-accent-soft text-accent;
}

.member-avatar.quiet {
  @apply bg-bg-hover text-text-tertiary;
}

.member-name {
  @apply flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12.5px];
}

.member-name.muted {
  @apply text-text-tertiary;
}

.member-role {
  @apply flex-none text-[10.5px] text-text-tertiary;
}

.swatches {
  @apply flex items-center gap-2;
}

.swatch {
  @apply w-[22px] h-[22px] rounded-full border-none cursor-pointer p-0;
}

.swatch.chosen {
  box-shadow:
    0 0 0 2px var(--bg-panel),
    0 0 0 3.5px var(--tint);
}

.failure {
  @apply m-0 mb-3 text-[12px] text-red;
}

.foot {
  @apply flex items-center justify-between pt-2.5 mt-1 border-t border-border;
}

.remove {
  @apply border-none bg-transparent text-red text-[12.5px] cursor-pointer p-1 rounded-[5px];
  font: inherit;
}

.remove:hover {
  @apply bg-red-soft;
}

.remove.disabled {
  @apply text-text-tertiary cursor-default;
}

.remove.disabled:hover {
  @apply bg-transparent;
}
</style>
