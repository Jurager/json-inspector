<script setup lang="ts">
import { computed } from 'vue'
import type { Workspace } from '../../../bindings/json-inspector/internal/domain'
import { tintOf } from './palette'
import { workspaceInitial } from '../../stores/workspaces'

// The square with a letter in it, in two sizes: the one in the titlebar pill and the one in a menu
// row, which the mockup draws 16 and 20 pixels across.
const props = withDefaults(defineProps<{ workspace: Workspace; size?: 'sm' | 'md' }>(), {
  size: 'md',
})

const style = computed(() => ({ background: tintOf(props.workspace.color) }))
</script>

<template>
  <span class="avatar" :class="`avatar--${props.size}`" :style="style">
    {{ workspaceInitial(workspace) }}
  </span>
</template>

<style scoped>
@reference "../../style.css";

.avatar {
  @apply flex-none inline-flex items-center justify-center text-white font-bold;
  letter-spacing: 0.01em;
}

.avatar--sm {
  @apply w-4 h-4 rounded-[5px];
  font-size: 9px;
}

.avatar--md {
  @apply w-5 h-5 rounded-md;
  font-size: 10px;
}
</style>
