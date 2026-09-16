<script setup lang="ts">
import { SwitchRoot, SwitchThumb, type SwitchRootProps } from 'reka-ui'

// The handoff's switch: a 38x22 track with an 18px knob inset by 2px, and green when on. Green is
// the design's own colour for "this is on" — it is not the accent, and the two must not be confused
// with each other on a settings row where both appear.
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
  width: 38px;
  height: 22px;
  background: var(--border-strong);
  transition: background 0.15s ease;
}

.switch[data-state='checked'] {
  background: var(--green);
}

.switch:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.thumb {
  @apply block rounded-full;
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
  transition: transform 0.15s ease;
}

/* 38 - 2 - 18 - 2 */
.switch[data-state='checked'] .thumb {
  transform: translateX(16px);
}

@media (prefers-reduced-motion: reduce) {
  .switch,
  .thumb {
    transition: none;
  }
}
</style>
