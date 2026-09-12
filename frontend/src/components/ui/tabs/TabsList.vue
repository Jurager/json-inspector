<script setup lang="ts">
import { TabsList, type TabsListProps } from 'reka-ui'

// inheritAttrs is off, so the caller's class has to be bound by hand: a
// fallthrough class would land on the reka-ui component rather than this element.
const props = defineProps<TabsListProps>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <TabsList v-bind="{ ...props, ...$attrs }" class="tabs">
    <slot />
  </TabsList>
</template>
