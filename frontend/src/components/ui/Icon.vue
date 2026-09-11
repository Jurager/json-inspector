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
  Menu,
  Info,
  Sparkles,
  Folder,
  Settings2,
  List,
  Lock,
  Plus,
  Minus,
  Trash2,
  Search,
  Eye,
  EyeOff,
  CornerDownRight,
  Pencil,
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
  menu: Menu,
  info: Info,
  sparkles: Sparkles,
  folder: Folder,
  'settings-2': Settings2,
  list: List,
  lock: Lock,
  plus: Plus,
  minus: Minus,
  trash: Trash2,
  search: Search,
  eye: Eye,
  'eye-off': EyeOff,
  inherit: CornerDownRight,
  pencil: Pencil,
}

const icon = computed(() => ICONS[props.name] ?? ArrowUpRight)
const size = computed(() => props.size ?? 16)
</script>

<template>
  <component :is="icon" :size="size" :stroke-width="1.5" aria-hidden="true" />
</template>
