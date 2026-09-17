<script setup lang="ts">
import { DropdownMenuItem } from 'reka-ui'
import { DropdownMenuContent } from '../ui/dropdown-menu'
import Icon from '../ui/Icon.vue'
import WorkspaceAvatar from './WorkspaceAvatar.vue'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { useWorkspaceCounts } from '../../composables/useWorkspaceCounts'
import { useMessages } from '../../i18n'
import { switchWorkspace } from '../../composables/useWorkspaceSwitch'

// The panel under the titlebar's chip: every space, in the order they were made, with what each of
// them holds under its name. Choosing one moves the whole window; the two buttons at the foot are the
// ways out of the panel — a new space, and the window that manages the ones there are.
const store = useWorkspacesStore()
const { t } = useMessages()
const { metaOf } = useWorkspaceCounts()

async function choose(id: string) {
  await switchWorkspace(id)
}
</script>

<template>
  <DropdownMenuContent class="ws-menu" align="start">
    <div class="ws-head">{{ t('workspaces.title') }}</div>

    <DropdownMenuItem
      v-for="workspace in store.list"
      :key="workspace.id"
      class="ws-row"
      :class="{ active: workspace.id === store.activeId }"
      @select="choose(workspace.id)"
    >
      <WorkspaceAvatar :workspace="workspace" />
      <span class="ws-text">
        <span class="ws-name" :class="{ strong: workspace.id === store.activeId }">
          {{ workspaceName(workspace) }}
        </span>
        <span class="ws-meta">{{ metaOf(workspace) }}</span>
      </span>
      <Icon
        v-if="workspace.id === store.activeId"
        name="check"
        :size="15"
        :stroke-width="2.6"
        class="ws-check"
      />
    </DropdownMenuItem>

    <div class="ws-divider"></div>

    <!-- The two ways out are menu items and not plain buttons: a panel that stayed open behind the
         window it just opened would be a second thing on screen, and closing on the way out is what
         an item does. `as-child` so the links keep the drawing's own look rather than the menu
         item's. -->
    <div class="ws-foot">
      <DropdownMenuItem as-child @select="store.openCreate()">
        <button type="button" class="ws-link">
          <Icon name="plus" :size="15" :stroke-width="2.2" />
          {{ t('workspaces.create') }}
        </button>
      </DropdownMenuItem>
      <DropdownMenuItem as-child @select="store.openSheet()">
        <button type="button" class="ws-link quiet">
          <Icon name="settings-2" :size="15" :stroke-width="1.8" />
          {{ t('workspaces.configure') }}
        </button>
      </DropdownMenuItem>
    </div>
  </DropdownMenuContent>
</template>

<style scoped>
@reference "../../style.css";

.ws-head {
  @apply px-2.5 pt-1.5 pb-2 text-[11px] font-semibold uppercase tracking-[0.07em] text-text-tertiary;
}

.ws-row {
  @apply flex items-center gap-[11px] w-full py-[9px] px-2.5 border-none rounded-[9px] bg-transparent text-text text-left cursor-pointer;
  font: inherit;
  --wails-draggable: no-drag;
}

.ws-row:hover,
.ws-row[data-highlighted] {
  @apply bg-bg-hover;
  outline: none;
}

.ws-row.active {
  @apply bg-accent-soft;
}

.ws-text {
  @apply flex-1 min-w-0 flex flex-col;
}

.ws-name {
  @apply overflow-hidden text-ellipsis whitespace-nowrap text-[13.5px];
}

.ws-name.strong {
  @apply font-semibold;
}

/* What the space holds, where the drawing puts the line about a shared one: eleven and a half pixels,
   in the tertiary ink, because it is a reading and not a name. */
.ws-meta {
  @apply text-[11.5px] text-text-tertiary overflow-hidden text-ellipsis whitespace-nowrap;
}

.ws-check {
  @apply flex-none text-accent;
}

/* The mockup draws the footer's rule as a border on the footer itself: three pixels above it, eight
   below it, and the buttons inset six. */
.ws-divider {
  @apply h-px mx-1.5 mt-[3px] bg-border;
}

.ws-foot {
  @apply flex items-center justify-between pt-2 pb-0.5 px-1.5;
}

.ws-link {
  @apply inline-flex items-center gap-[7px] border-none bg-transparent text-accent text-[13.5px] cursor-pointer h-[30px] px-2.5 rounded-[7px];
  font: inherit;
}

/* The second door out of the panel: a plain one, and the same height as the first. */
.ws-link.quiet {
  @apply text-[13px] text-text-secondary;
}

.ws-link:hover {
  @apply bg-accent-soft;
}

.ws-link.quiet:hover {
  @apply bg-bg-hover text-text;
}
</style>
