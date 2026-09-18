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
  <!-- The counts under the names are a reading of what other features hold, and nothing tells this
       panel when one of them changes: it reads them again every time it is opened. -->
  <DropdownMenu v-if="store.active" @update:open="(open) => open && store.load()">
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

/* The pill keeps the bar's own recipe — 30px, 13px — and adds room for the avatar beside the
   name. The name is capped so a long one cannot push the environment out of the middle. */
.workspace-pill {
  padding: 0 10px;
}

.workspace-name {
  @apply max-w-[140px] overflow-hidden text-ellipsis whitespace-nowrap;
}

.workspace-chevron {
  @apply text-text-tertiary;
}
</style>
