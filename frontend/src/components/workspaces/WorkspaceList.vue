<script setup lang="ts">
import WorkspaceAvatar from './WorkspaceAvatar.vue'
import Icon from '../ui/Icon.vue'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { useWorkspaceCounts } from '../../composables/useWorkspaceCounts'
import { useMessages } from '../../i18n'

// The rail of the manager window: every workspace as a row, and the one way to make another. What a
// space holds is the second line — the same reading the window's Contents block draws, said in one
// line because a row has one line to say it in.
const emit = defineEmits<{ (e: 'select', id: string): void; (e: 'create'): void }>()

const store = useWorkspacesStore()
const { t } = useMessages()
const { metaOf } = useWorkspaceCounts()
</script>

<template>
  <div class="sheet-side">
    <div class="side-rows">
      <button
        v-for="workspace in store.list"
        :key="workspace.id"
        type="button"
        class="side-row"
        :class="{ active: workspace.id === store.editedId }"
        @click="emit('select', workspace.id)"
      >
        <WorkspaceAvatar :workspace="workspace" size="rail" />
        <span class="side-text">
          <span class="side-name">{{ workspaceName(workspace) }}</span>
          <span class="side-meta">{{ metaOf(workspace) }}</span>
        </span>
      </button>
    </div>

    <div class="side-foot">
      <button type="button" class="side-new" @click="emit('create')">
        <Icon name="plus" :size="14" :stroke-width="2.2" />
        {{ t('workspaces.create') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.sheet-side {
  @apply flex-none w-[236px] flex flex-col p-2 pb-0 border-r;
  background: var(--glass-sheet-side);
  border-color: var(--border);
}

/* The rows scroll and the button does not: a person with twenty workspaces would otherwise push the
   way to make another one off the leaf. No gap between them — the drawing leaves none, and a gap
   turns one block of names into a list of separate things. */
.side-rows {
  @apply flex-1 min-h-0 overflow-y-auto flex flex-col;
}

.side-row {
  @apply flex items-center gap-2.5 w-full text-left border-0 rounded-lg
         bg-transparent cursor-pointer text-text;
  font-family: inherit;
  padding: 8px 9px;
  --wails-draggable: no-drag;
}

.side-row:hover {
  @apply bg-bg-hover;
}

.side-row.active {
  @apply bg-accent-soft;
}

.side-text {
  @apply flex-1 min-w-0 flex flex-col gap-px;
}

.side-name {
  @apply text-[13px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.side-row.active .side-name {
  @apply font-semibold;
}

.side-meta {
  @apply text-[11px] text-text-tertiary overflow-hidden text-ellipsis whitespace-nowrap;
}

/* The same 48px bar as the panes' own feet, which is what keeps the button in it level with theirs:
   10px of air around a 28px button puts its centre 24 above the bottom, and a 48px bar centring what
   stands in it puts theirs 23.5 — half a pixel, which is a hairline nobody can see. */
.side-foot {
  @apply flex-none;
  padding: 10px 2px;
  border-top: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.side-new {
  @apply flex items-center gap-[7px] h-7 px-2 border-0 rounded-md cursor-pointer
         bg-transparent text-accent;
  font-family: inherit;
  font-size: 12.5px;
  font-weight: 500;
}

.side-new:hover {
  @apply bg-accent-soft;
}
</style>
