<script setup lang="ts">
import { TabsTrigger, type TabsTriggerProps } from 'reka-ui'

// The .tab rule in style.css keys on data-state="active", which reka-ui sets;
// inheritAttrs is off, so props and attrs are forwarded by hand.
const props = defineProps<TabsTriggerProps>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <TabsTrigger v-bind="{ ...props, ...$attrs }" class="tab">
    <slot />
  </TabsTrigger>
</template>
