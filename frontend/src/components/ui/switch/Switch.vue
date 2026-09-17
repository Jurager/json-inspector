<script setup lang="ts">
import { SwitchRoot, SwitchThumb, type SwitchRootProps } from 'reka-ui'

// The handoff's switch: a 38x22 track with an 18px knob inset by 2px, and the accent when on under a
// groove when off. The accent and not a colour of its own: the drawing that put green here drew the
// switch beside buttons wearing the accent, and a second meaning for "on" is one the window does not
// need — the one place green still says something is a tag's tone.
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
  background: var(--bg-hover);
  transition: background 0.15s ease;
}

.switch[data-state='checked'] {
  background: var(--accent);
}

.switch:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* A switch that is drawn and not offered — a setting whose feature is not there yet — keeps its
   place in the row and is dimmed, like the segment's `soon` option. */
.switch:disabled {
  @apply opacity-50 cursor-default;
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
