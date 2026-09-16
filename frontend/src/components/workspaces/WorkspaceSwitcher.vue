<script setup lang="ts">
import { Button } from '../ui/button'
import { DropdownMenu, DropdownMenuTrigger } from '../ui/dropdown-menu'
import { useWorkspacesStore, workspaceName } from '../../stores/workspaces'
import { useMessages } from '../../i18n'
import Icon from '../ui/Icon.vue'
import WorkspaceAvatar from './WorkspaceAvatar.vue'
import WorkspaceMenu from './WorkspaceMenu.vue'

const store = useWorkspacesStore()

const { t } = useMessages()
</script>

<template>
  <DropdownMenu v-if="store.active">
    <DropdownMenuTrigger as-child>
      <Button
        class="workspace-pill"
        :title="t('workspaces.switcher', { name: workspaceName(store.active) })"
      >
        <WorkspaceAvatar :workspace="store.active" size="sm" />
        <span class="workspace-name">{{ workspaceName(store.active) }}</span>
        <Icon name="chevron-down" :size="10" :stroke-width="2.2" class="workspace-chevron" />
      </Button>
    </DropdownMenuTrigger>
    <WorkspaceMenu />
  </DropdownMenu>
</template>

<style scoped>
@reference "../../style.css";

/* The chip's own recipe, with the mockup's tighter left inset: the avatar sits in the corner the
   circle of an environment would, and the text keeps the same room. */
.workspace-pill {
  padding: 0 8px 0 6px;
}

.workspace-name {
  @apply max-w-[140px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.workspace-chevron {
  @apply text-text-tertiary;
}
</style>
