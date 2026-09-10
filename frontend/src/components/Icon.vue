<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import {
  ArrowUpRight,
  ArrowDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronDown,
  ArrowRight,
  ArrowLeft,
  X,
  Check,
  CircleDot,
  ArrowLeftRight,
  ChevronsLeft,
  ChevronsRight,
} from 'lucide-vue-next'

const props = defineProps<{ name: string; size?: number }>()

const ICONS: Record<string, Component> = {
  'arrow-up-right': ArrowUpRight,
  record: CircleDot,
  'arrow-down': ArrowDown,
  'chevron-left': ChevronLeft,
  'chevron-right': ChevronRight,
  'chevron-up': ChevronUp,
  'chevron-down': ChevronDown,
  'arrow-right': ArrowRight,
  'arrow-left': ArrowLeft,
  xmark: X,
  check: Check,
  compare: ArrowLeftRight,
  'chevrons-left': ChevronsLeft,
  'chevrons-right': ChevronsRight,
}

const icon = computed(() => ICONS[props.name] ?? ArrowUpRight)
const size = computed(() => props.size ?? 16)
</script>

<template>
  <component :is="icon" :size="size" :stroke-width="1.5" aria-hidden="true" />
</template>
