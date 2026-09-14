<script setup lang="ts">
import { computed, defineComponent, h } from 'vue'
import type { Component, VNode } from 'vue'
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
  Clock,
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
  Pause,
  Square,
  Download,
  Upload,
  CodeXml,
  Contrast,
  Globe,
  User,
  createLucideIcon,
} from 'lucide-vue-next'

const props = defineProps<{
  name: string
  size?: number
  strokeWidth?: number
  filled?: boolean
}>()

// The two glyphs that mean their word with a solid shape rather than an outline are drawn here, on
// the same 24-unit grid as the rest: lucide's icons always paint a stroke in the current colour, and
// around a solid shape a stroke is more ink — the pause bars come out a third wider than they were
// asked for and the play triangle loses its point. Their size is the box, and the stroke width is
// taken and dropped, because a solid glyph has no stroke to weigh.
function solidGlyph(name: string, draw: () => VNode[]): Component {
  return defineComponent({
    name,
    props: { size: { type: Number, default: 16 }, strokeWidth: { type: Number, default: 1.5 } },
    setup: (own) => () =>
      h(
        'svg',
        {
          xmlns: 'http://www.w3.org/2000/svg',
          viewBox: '0 0 24 24',
          width: own.size,
          height: own.size,
          fill: 'currentColor',
          stroke: 'none',
        },
        draw()
      ),
  })
}

// The markup is the mockup's, not a redrawing of it.
const SOLID_ICONS: Record<string, Component> = {
  pause: solidGlyph('pause-solid', () => [
    h('rect', { x: 6, y: 5, width: 4, height: 14, rx: 1 }),
    h('rect', { x: 14, y: 5, width: 4, height: 14, rx: 1 }),
  ]),
  play: solidGlyph('play-solid', () => [h('path', { d: 'M7 5v14l12-7Z' })]),
}

// The three positions of the list panel are not in lucide's set, so they are built from the same
// factory the rest of it comes from: the window frame, with one column filled in, or struck through
// where there is no panel. The band carries no stroke of its own, so that its edge does not double
// up with the frame's.
const panelMark = (side: 'left' | 'right') =>
  createLucideIcon(`panel-${side}`, [
    ['rect', { x: '3', y: '4', width: '18', height: '16', rx: '2' }],
    [
      'rect',
      {
        x: side === 'left' ? '3' : '14',
        y: '4',
        width: '7',
        height: '16',
        rx: '1',
        fill: 'currentColor',
        'fill-opacity': '0.35',
        stroke: 'none',
      },
    ],
  ])

const PANEL_LEFT = panelMark('left')
const PANEL_RIGHT = panelMark('right')
const PANEL_HIDDEN = createLucideIcon('panel-hidden', [
  ['rect', { x: '3', y: '4', width: '18', height: '16', rx: '2' }],
  ['path', { d: 'M4 5l16 14' }],
])

const ICONS: Record<string, Component> = {
  'arrow-up-right': ArrowUpRight,
  clock: Clock,
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
  pause: Pause,
  stop: Square,
  'panel-left': PANEL_LEFT,
  'panel-right': PANEL_RIGHT,
  'panel-hidden': PANEL_HIDDEN,
  download: Download,
  upload: Upload,
  contrast: Contrast,
  globe: Globe,
  user: User,
}

// `filled` asks for the solid version of a glyph, and only the two above have one: a name without a
// twin is drawn as the outline it always was.
const icon = computed(
  () => (props.filled ? SOLID_ICONS[props.name] : undefined) ?? ICONS[props.name] ?? ArrowUpRight
)
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
  <component
    :is="icon"
    :size="pixelSize"
    :stroke-width="strokeWidth ?? 1.5"
    aria-hidden="true"
  />
</template>
