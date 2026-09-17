<script setup lang="ts">
import { DropdownMenuItem } from 'reka-ui'
import { DropdownMenuContent } from '../ui/dropdown-menu'
import Icon from '../ui/Icon.vue'
import WorkspaceAvatar from './WorkspaceAvatar.vue'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { useMessages } from '../../i18n'
import { switchWorkspace } from '../../composables/useWorkspaceSwitch'
import { WorkspaceKind } from '../../../bindings/json-inspector/internal/domain'

const store = useWorkspacesStore()

const { t } = useMessages()

async function choose(id: string) {
  await switchWorkspace(id)
}

function configure(id: string) {
  store.openSettings(id)
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
        <!-- What the design draws under a team's name. Nothing makes one yet, so this line is what
             will say a space is shared the day there is something to share it with. -->
        <span v-if="workspace.kind === WorkspaceKind.WorkspaceTeam" class="ws-kind">
          {{ t('workspaces.teamKind', { count: 0 }) }}
        </span>
      </span>
      <Icon
        v-if="workspace.id === store.activeId"
        name="check"
        :size="12"
        :stroke-width="2.4"
        class="ws-check"
      />
    </DropdownMenuItem>

    <div class="ws-divider"></div>

    <div class="ws-foot">
      <button class="ws-link" @click="store.openCreate()">{{ t('workspaces.create') }}</button>
      <button class="ws-link quiet" @click="configure(store.activeId)">
        <Icon name="settings-2" :size="12" :stroke-width="1.6" />
        {{ t('workspaces.configure') }}
      </button>
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

.ws-kind {
  @apply text-[11.5px] text-text-tertiary;
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
  @apply inline-flex items-center gap-1 border-none bg-transparent text-accent text-[13.5px] cursor-pointer h-[30px] px-2.5 rounded-[7px];
  font: inherit;
}

/* The second door out of the panel: a plain one, and the same height as the first. */
.ws-link-plain {
  @apply text-[13px] text-text-secondary;
}

.ws-link.quiet {
  @apply text-[11.5px] text-text-secondary gap-1.5;
}

.ws-link:hover {
  @apply bg-accent-soft;
}

.ws-link.quiet:hover {
  @apply bg-bg-hover text-text;
}
</style>
