<script setup lang="ts">
import { PopoverContent, PopoverPortal, type PopoverContentProps } from 'reka-ui'

// The panel look is the app's `.popover` class; a list of commands is
// ui/dropdown-menu instead.
//
// inheritAttrs is off and $attrs is bound on the content by hand: the root here
// is the Portal, and a class falling through onto a Teleport is dropped — which
// is what used to keep a caller's panel class (the width above all) off the panel.
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
