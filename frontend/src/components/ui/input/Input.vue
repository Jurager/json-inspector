<script setup lang="ts">
// The app's text field. reka-ui has no Input either, so this is the handoff's
// field written once: 32px tall with a 8px radius at md, 28px and 7px at sm,
// --bg-inset inside a --border, and on focus the accent border plus the 3px
// --accent-soft ring every field in the design carries.
//
// `bare` is the same element with none of that chrome: a field that lives
// inside a container drawing the frame itself (the filter pills, the command
// line's URL box). Layout stays with the caller in both variants.
import { ref } from 'vue'

withDefaults(
  defineProps<{
    modelValue?: string
    variant?: 'field' | 'bare'
    size?: 'sm' | 'md'
    type?: string
    mono?: boolean
  }>(),
  { variant: 'field', size: 'md', type: 'text' }
)

const emit = defineEmits<{ 'update:modelValue': [string] }>()

const el = ref<HTMLInputElement | null>(null)

// Callers only ever want to put the caret in it, so that is the whole exposed
// surface — `ref.focus()` instead of reaching past the component for the node.
defineExpose({ focus: () => el.value?.focus() })
</script>

<template>
  <input
    ref="el"
    :type="type"
    :value="modelValue"
    :class="['inp', `inp--${variant}`, `inp--${size}`, { 'inp--mono': mono }]"
    @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>

<style scoped>
@reference "../../../style.css";

.inp {
  @apply text-text outline-none;
  font-family: inherit;
  font-size: inherit;
  transition: border-color 0.12s ease, box-shadow 0.12s ease;
  --wails-draggable: no-drag;
}

.inp--mono {
  font-family: var(--mono);
}

.inp--field {
  background: var(--bg-inset);
  border: 1px solid var(--border);
}

.inp--field.inp--md {
  height: 32px;
  padding: 0 8px;
  border-radius: 8px;
  font-size: 13px;
}

.inp--field.inp--sm {
  height: 28px;
  padding: 0 8px;
  border-radius: 7px;
  font-size: 12px;
}

.inp--field:hover:not(:disabled) {
  border-color: var(--border-strong);
}

.inp--field:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.inp--field:disabled {
  @apply opacity-50 cursor-default;
}

.inp--bare {
  @apply border-0 bg-transparent;
}

.inp--bare.inp--sm {
  font-size: 12px;
}
</style>
