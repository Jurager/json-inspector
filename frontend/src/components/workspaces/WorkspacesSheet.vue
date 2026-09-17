<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import WorkspaceCreatePane from './WorkspaceCreatePane.vue'
import WorkspaceDetailPane from './WorkspaceDetailPane.vue'
import WorkspaceDangerSheet from './WorkspaceDangerSheet.vue'
import WorkspaceList from './WorkspaceList.vue'
import { useWorkspacesStore } from '../../stores/workspaces'
import { enterWorkspace } from '../../composables/useWorkspaceSwitch'
import { useSheetNotice } from '../../composables/useSheetNotice'
import { describeFailure, useMessages } from '../../i18n'

// The manager window: the spaces on the left and, on the right, either the form that makes a new one or
// the space being looked at. It holds the two things neither side can hold alone — which pane the
// right-hand side is showing, and the confirmation that stands over everything before a space is taken
// away — and it is also where a delete of the space *on screen* stops leaving the panels below showing
// a workspace that no longer exists.
const emit = defineEmits<{ (e: 'close'): void }>()

const store = useWorkspacesStore()
const { setNotice, clearNotice } = useSheetNotice()
const { t } = useMessages()

const detailRef = ref<InstanceType<typeof WorkspaceDetailPane> | null>(null)
const confirming = ref(false)

const creating = computed(() => store.creating)
const workspace = computed(() => store.edited)

function select(id: string) {
  store.edit(id)
  clearNotice()
}

// The counts are a reading of what other features hold, and nothing tells this window when a collection
// is made — so the window reads them again when it opens rather than drawing what the session started
// with.
onMounted(() => {
  clearNotice()
  void store.load()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

async function confirmDanger() {
  confirming.value = false
  const target = workspace.value
  if (!target) return

  const wasActive = target.id === store.activeId
  try {
    await store.remove(target.id)
  } catch (error) {
    setNotice(describeFailure(error))
    return
  }

  // The space on screen went with it: everything below this window belonged to it, so it is emptied and
  // read again — the same gesture as a switch, and the same function.
  if (wasActive) await enterWorkspace()

  // The pane moves to what is left rather than pointing at a row that is gone.
  store.edit(store.activeId)
}

function onKeydown(e: KeyboardEvent) {
  if (e.key !== 'Escape') return
  if (confirming.value) {
    confirming.value = false
    return
  }
  if (creating.value) {
    store.cancelCreating()
    return
  }
  if (detailRef.value?.cancelTop()) return
  emit('close')
}
</script>

<template>
  <div class="sheet-overlay">
    <div class="sheet">
      <div class="sheet-head">
        <span class="sheet-title">{{ t('workspaces.title') }}</span>
        <span class="head-spacer"></span>
        <button type="button" class="done" @click="emit('close')">{{ t('workspaces.done') }}</button>
      </div>

      <div class="sheet-body">
        <!-- The list steps aside while a new space is being made: the form is the only thing to do at
             that moment, and a row to click beside it would only take the room the name and the colour
             want. Cancel and Escape are the way back, and they are both on screen. -->
        <WorkspaceList v-if="!creating" @select="select" @create="store.openCreate()" />

        <WorkspaceCreatePane
          v-if="creating"
          @cancel="store.cancelCreating()"
          @created="clearNotice()"
        />
        <WorkspaceDetailPane v-else ref="detailRef" @danger="confirming = true" />
      </div>

      <WorkspaceDangerSheet
        v-if="confirming"
        @close="confirming = false"
        @confirm="confirmDanger"
      />
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-overlay {
  @apply fixed inset-0 z-1500 flex items-center justify-center;
  padding: 28px;
  background: rgba(0, 0, 0, 0.22);
}

/* The whole window is one acrylic leaf — blurring only the toolbar left a glass strip lying on an
   ordinary card. The strips inside it are veils of the same shade, not a fill of their own, and the
   lines between them are the window's own hairline rather than the leaf's: the drawing draws the
   head and the rail with `--line`, and a sheet's inner edges are a touch lighter than its outer. */
.sheet {
  @apply flex flex-col w-full h-full max-w-[900px] max-h-[580px] rounded-[14px] overflow-hidden;
  background: var(--glass-sheet);
  backdrop-filter: var(--blur-sheet);
  box-shadow: var(--glass-sheet-shadow), 0 0 0 1px var(--glass-overlay-border);
}

.sheet-head {
  @apply flex-none flex items-center gap-3 h-[50px] px-4 border-b;
  background: var(--glass-sheet-head);
  border-color: var(--border);
}

.sheet-title {
  @apply text-sm font-semibold;
}

/* The head's one button: a field's fill and hairline rather than a bar's, because it ends a window
   rather than standing in a row of controls. */
.done {
  @apply flex-none h-[30px] px-3.5 rounded-lg cursor-pointer text-text text-[13px] font-medium;
  font-family: inherit;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
}

.done:hover {
  @apply bg-bg-hover;
}

.sheet-body {
  @apply flex flex-1 min-h-0;
}
</style>
