<script setup lang="ts">
import { computed } from 'vue'
import type { JsonApiDocument } from '../../lib/jsonapi'
import { validateDocument, type SchemaCheck } from '../../lib/schema'

const props = defineProps<{ doc: JsonApiDocument }>()

const checks = computed<SchemaCheck[]>(() => validateDocument(props.doc))
</script>

<template>
  <div class="tests">
    <div v-if="checks.length === 0" class="tests-ok">
      <span class="dot dot-green"></span>
      <span>Документ соответствует JSON:API</span>
    </div>
    <div v-else class="check-list">
      <div v-for="(c, i) in checks" :key="i" class="check-row">
        <span class="dot" :class="c.status === 'error' ? 'dot-red' : c.status === 'warn' ? 'dot-orange' : 'dot-green'"></span>
        <span class="check-msg">{{ c.message }}</span>
        <span v-if="c.path" class="check-path mono">{{ c.path }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
@reference "../../style.css";

.tests {
  @apply p-4 flex flex-col gap-2;
}

.tests-ok {
  @apply flex items-center gap-2 text-xs text-text;
}

.check-list {
  @apply flex flex-col gap-2;
}

.check-row {
  @apply flex items-baseline gap-2 text-xs;
}

.check-msg {
  @apply text-text;
}

.check-path {
  @apply text-text-tertiary break-all;
}

.dot {
  @apply w-[7px] h-[7px] rounded-full flex-none;
}

.dot-green {
  background: var(--green);
}

.dot-orange {
  background: var(--orange);
}

.dot-red {
  background: var(--red);
}
</style>
