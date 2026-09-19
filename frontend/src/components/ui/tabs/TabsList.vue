<script setup lang="ts">
import { ref } from 'vue'
import { TabsList, type TabsListProps } from 'reka-ui'
import { useSlidingPill } from '../../../composables/useSlidingPill'

// inheritAttrs is off, so the caller's class has to be bound by hand: a
// fallthrough class would land on the reka-ui component rather than this element.
const props = defineProps<TabsListProps>()

defineOptions({ inheritAttrs: false })

// The rule under the chosen tab slides from the tab that was chosen to the one that is, as every
// other choice in the window moves. It is hosted here rather than in each bar because this is the
// component all of them go through — and because reka says which tab is on with an attribute, which
// the composable watches on the list itself.
const listEl = ref<HTMLElement | null>(null)
const { style: markStyle, ready: markReady } = useSlidingPill(listEl, '.tab[data-state="active"]', () => null)
</script>

<template>
  <TabsList ref="listEl" v-bind="{ ...props, ...$attrs }" class="tabs">
    <span class="tab-mark slide-mark" :class="{ ready: markReady }" :style="markStyle"></span>
    <slot />
  </TabsList>
</template>
