<script setup lang="ts">
import { PopoverContent, PopoverPortal, type PopoverContentProps } from 'reka-ui'

// The panel look is the app's `.popover`; a list of commands is ui/dropdown-menu. $attrs is
// bound by hand: the root is the Portal, and a class on a Teleport is dropped.
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
