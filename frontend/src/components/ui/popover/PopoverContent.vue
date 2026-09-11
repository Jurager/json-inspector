<script setup lang="ts">
import { PopoverContent, PopoverPortal, type PopoverContentProps } from 'reka-ui'

// Placement, focus, Escape, click-outside and ARIA are reka-ui's; the panel look
// is the app's .popover class, so a migrated popover keeps its appearance. This
// is for content that is a panel — ui/dropdown-menu is for a list of commands.
//
// inheritAttrs is off and $attrs is bound on the content by hand: the root here
// is the Portal, and a class that falls through onto a Teleport is dropped. That
// is what kept every caller's panel class — the width above all — off the panel.
defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<PopoverContentProps>(), {
  sideOffset: 6,
  align: 'start',
})
</script>

<template>
  <PopoverPortal>
    <PopoverContent v-bind="{ ...props, ...$attrs }" class="popover">
      <slot />
    </PopoverContent>
  </PopoverPortal>
</template>
