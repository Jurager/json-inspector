<script setup lang="ts">
import { DropdownMenuContent, DropdownMenuPortal, type DropdownMenuContentProps } from 'reka-ui'

// inheritAttrs is off and $attrs is bound on the content by hand: the root here
// is the Portal, and a class that falls through onto a Teleport is dropped.
defineOptions({ inheritAttrs: false })

const props = withDefaults(defineProps<DropdownMenuContentProps>(), {
  sideOffset: 6,
  align: 'end',
})
</script>

<template>
  <DropdownMenuPortal>
    <DropdownMenuContent v-bind="{ ...props, ...$attrs }" class="menu">
      <slot />
    </DropdownMenuContent>
  </DropdownMenuPortal>
</template>
