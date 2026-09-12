<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot, type CheckboxRootProps } from 'reka-ui'
import Icon from '../Icon.vue'

// The handoff's row checkbox: 13x13, 3px radius.
//
// `tone` is ours, and a checked box means two things here: "this row is on"
// (accent) or "this value is a credential" (the sheet's "секрет" orange). It is
// kept out of the forwarded props, or it would land on the button as an
// attribute.
const { tone = 'accent', ...rootProps } = defineProps<CheckboxRootProps & { tone?: 'accent' | 'secret' }>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <CheckboxRoot v-bind="{ ...rootProps, ...$attrs }" class="check" :class="{ secret: tone === 'secret' }">
    <!-- Mounted only while checked, so the mark needs no v-if of its own. -->

    <CheckboxIndicator>
      <Icon name="check" :size="9" />
    </CheckboxIndicator>
  </CheckboxRoot>
</template>

<style scoped>
@reference "../../../style.css";

.check {
  @apply flex-none flex items-center justify-center cursor-pointer text-white;
  width: 13px;
  height: 13px;
  border-radius: 3px;
  border: 1px solid var(--border-strong);
  background: var(--bg-panel);
}

.check[data-state='checked'] {
  border-color: var(--accent);
  background: var(--accent);
}

.check.secret[data-state='checked'] {
  border-color: var(--orange);
  background: var(--orange);
}
</style>
