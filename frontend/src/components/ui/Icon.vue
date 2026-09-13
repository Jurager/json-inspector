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
  Sun,
  Moon,
  Monitor,
  Link,
  Bookmark,
  Play,
  Square,
  Download,
  Upload,
  CodeXml,
  Contrast,
  Globe,
} from 'lucide-vue-next'

const props = defineProps<{ name: string; size?: number; strokeWidth?: number }>()

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
  'code-xml': CodeXml,
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
  sun: Sun,
  moon: Moon,
  monitor: Monitor,
  link: Link,
  bookmark: Bookmark,
  play: Play,
  stop: Square,
  download: Download,
  upload: Upload,
  contrast: Contrast,
  globe: Globe,
}

const icon = computed(() => ICONS[props.name] ?? ArrowUpRight)
const pixelSize = computed(() => props.size ?? 16)

// `size` is the box the glyph is drawn in, and glyphs do not fill their box equally: lucide's `trash`
// spans most of its 24-unit viewBox, `plus` 14 of them and `xmark` 12. At one nominal size the three
// therefore come out at 10.6, 8.4 and 6.9 pixels of ink — so a plus beside a bin reads as a text
// glyph rather than as an icon, which is how it looked. The ones that fill less are asked for at a
// larger size instead; that is why `plus` is 16 where a bin is 13. Both numbers are the size of the
// box, not of the mark, and changing either means looking at the pair, not at the number.
//
// The stroke needs no such care: it is scaled by size/24, so one default thickness gives every icon
// the same line — which is why nothing here overrides `stroke-width`.
</script>

<template>
  <component :is="icon" :size="pixelSize" :stroke-width="strokeWidth ?? 1.5" aria-hidden="true" />
</template>
