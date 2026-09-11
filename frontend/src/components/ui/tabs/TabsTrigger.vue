<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps } from 'reka-ui'

// reka-ui marks the chosen trigger with data-state="active", which is what the
// .tab rule in style.css answers to. Props are forwarded by hand along with the
// attributes, since inheritAttrs is off: `value` is a prop, not an attr.
const props = defineProps<TabsTriggerProps>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <TabsTrigger v-bind="{ ...props, ...$attrs }" class="tab">
    <slot />
  </TabsTrigger>
</template>
