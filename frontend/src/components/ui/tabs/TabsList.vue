<script setup lang="ts">
import { TabsList, type TabsListProps } from 'reka-ui'

// Roving tabindex, arrow keys and the aria wiring are reka-ui's; the strip's
// look is the app's .tabs class. The wrapper adds its own class, so the caller's
// has to be bound by hand — a fallthrough class would land on the reka-ui
// component rather than on this element.
const props = defineProps<TabsListProps>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <TabsList v-bind="{ ...props, ...$attrs }" class="tabs">
    <slot />
  </TabsList>
</template>
