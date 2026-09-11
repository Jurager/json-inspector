<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot, type CheckboxRootProps } from 'reka-ui'
import Icon from '../Icon.vue'

// The button role, the checked state, the Space key and the ARIA wiring are
// reka-ui's; the 13x13 box with a 3px radius is the handoff's row checkbox.
//
// `tone` exists because a checked box means two different things here: "this row
// is on" (accent) and "this value is a credential" — the app's orange, the same
// colour the sheet's "секрет" tag uses.
// `tone` is ours, not reka-ui's, so it is taken out of the props that are
// forwarded — spread in, it would land on the button as an attribute.
const { tone = 'accent', ...rootProps } = defineProps<CheckboxRootProps & { tone?: 'accent' | 'secret' }>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <CheckboxRoot v-bind="{ ...rootProps, ...$attrs }" class="check" :class="{ secret: tone === 'secret' }">
    <!-- The indicator is only mounted while checked, so the mark needs no
         v-if of its own. -->
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
