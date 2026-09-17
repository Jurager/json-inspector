<script setup lang="ts">
import { computed } from 'vue'
import type { Workspace } from '../../../bindings/json-inspector/internal/domain'
import { tintOf } from './palette'
import { workspaceInitial } from '../../stores/workspaces'

// The square with a letter in it, in the three sizes the drawing asks for: the titlebar pill, a row
// of the switcher, and a row of the manager's rail — the last two a shade apart, and each the size
// its own drawing gives.
const props = withDefaults(
  defineProps<{ workspace: Workspace; size?: 'sm' | 'md' | 'rail' }>(),
  { size: 'md' }
)

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
  /* The line box keeps room for the descenders a capital never reaches, so a single letter sits
     about half of that below the middle of the square. Padding at the foot shrinks the box the line
     is centred in, which lifts it by half the padding — and in em, so every size is the same. */
  padding-bottom: 0.09em;
}

.avatar--sm {
  @apply w-[18px] h-[18px] rounded-[5px];
  font-size: 11px;
}

.avatar--md {
  @apply w-6 h-6 rounded-[7px];
  font-size: 11px;
}

/* The rail's own: the same 22px the environments rail draws its avatars at, so the two rails of the
   two windows are one measure. */
.avatar--rail {
  @apply w-[22px] h-[22px] rounded-md;
  font-size: 10.5px;
}
</style>
