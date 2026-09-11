<script setup lang="ts">
import { SwitchRoot, SwitchThumb, type SwitchRootProps } from 'reka-ui'

// role="switch", aria-checked, the Space/Enter key and the label wiring are
// reka-ui's. Proportions follow the switch the project already had — the
// extension popup's: a 44x26 track with a 20px knob inset by 3px.
//
// No call site in the app yet: the environments sheet's "секрет/текст" is a tag
// in the handoff, not a toggle. It is here for the first real binary setting.
const props = defineProps<SwitchRootProps>()

defineOptions({ inheritAttrs: false })
</script>

<template>
  <SwitchRoot v-bind="{ ...props, ...$attrs }" class="switch">
    <SwitchThumb class="thumb" />
  </SwitchRoot>
</template>

<style scoped>
@reference "../../../style.css";

.switch {
  @apply relative flex-none rounded-full cursor-pointer;
  width: 44px;
  height: 26px;
  background: var(--border-strong);
  transition: background 0.2s ease;
}

.switch[data-state='checked'] {
  background: var(--accent);
}

.thumb {
  @apply block rounded-full;
  width: 20px;
  height: 20px;
  margin: 3px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
  transition: transform 0.2s ease;
}

/* 44 - 3 - 20 - 3 */
.switch[data-state='checked'] .thumb {
  transform: translateX(18px);
}
</style>
