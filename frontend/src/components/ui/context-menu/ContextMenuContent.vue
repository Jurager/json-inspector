<script setup lang="ts">
import { ContextMenuContent, ContextMenuPortal, type ContextMenuContentProps } from 'reka-ui'

// The same menu as the dropdown's, opened by a right click — the design's `.menu` is one thing, and
// the two only differ in what opens them.
defineOptions({ inheritAttrs: false })

const props = defineProps<ContextMenuContentProps>()
</script>

<template>
  <ContextMenuPortal>
    <!-- `closeAutoFocus` is prevented: a menu item here opens a row for the user to type in — a
         rename, a new request — and reka-ui's habit of handing focus back to the row that was
         right-clicked would take it straight out of the field that just appeared. -->
    <ContextMenuContent v-bind="{ ...props, ...$attrs }" class="menu" @close-auto-focus.prevent>
      <slot />
    </ContextMenuContent>
  </ContextMenuPortal>
</template>
