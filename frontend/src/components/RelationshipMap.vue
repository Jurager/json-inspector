<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  forceCenter,
  forceCollide,
  forceLink,
  forceManyBody,
  forceSimulation,
  type Simulation,
  type SimulationLinkDatum,
  type SimulationNodeDatum,
} from 'd3-force'
import type { jsonapi } from '../../wailsjs/go/models'

interface MapNode extends SimulationNodeDatum {
  key: string
  type: string
  id: string
  label: string
  source: string // data | included | missing
  related?: string
}

interface MapLink extends SimulationLinkDatum<MapNode> {
  rel: string
  external?: boolean
  source: MapNode
  target: MapNode
}

const props = defineProps<{
  graph: jsonapi.Graph | null
}>()

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'fetch', url: string): void
}>()

const svgEl = ref<SVGSVGElement | null>(null)
const nodes = ref<MapNode[]>([])
const links = ref<MapLink[]>([])
const view = reactive({ x: 0, y: 0, k: 1 })
const tick = ref(0)

let simulation: Simulation<MapNode, MapLink> | null = null
let resizeObserver: ResizeObserver | null = null
let size = { width: 800, height: 500 }

function currentSize() {
  if (svgEl.value) {
    const w = svgEl.value.clientWidth
    const h = svgEl.value.clientHeight
    if (w > 0 && h > 0) {
      size = { width: w, height: h }
    }
  }
  return size
}

function build() {
  if (!props.graph || !props.graph.nodes || props.graph.nodes.length === 0) return
  simulation?.stop()

  const { width, height } = currentSize()
  const rawNodes = props.graph.nodes
  const nodeMap = new Map<string, MapNode>()

  // Seed positions around a circle so the simulation spreads from a readable
  // starting state instead of everything piling up at the origin.
  const count = rawNodes.length
  const seedRadius = Math.max(160, count * 26)
  const initialNodes: MapNode[] = rawNodes.map((n, i) => {
    const angle = (i / Math.max(1, count)) * 2 * Math.PI
    const node: MapNode = {
      key: n.key,
      type: n.type,
      id: n.id,
      label: n.label,
      source: n.source,
      related: n.related,
      x: width / 2 + Math.cos(angle) * seedRadius,
      y: height / 2 + Math.sin(angle) * seedRadius,
    }
    nodeMap.set(n.key, node)
    return node
  })

  const initialLinks: MapLink[] = (props.graph.edges ?? [])
    .filter((e) => nodeMap.has(e.from) && nodeMap.has(e.to))
    .map((e) => ({
      rel: e.rel,
      external: e.external,
      source: nodeMap.get(e.from)!,
      target: nodeMap.get(e.to)!,
    }))

  nodes.value = initialNodes
  links.value = initialLinks

  simulation = forceSimulation<MapNode>(initialNodes)
    .force(
      'link',
      forceLink<MapNode, MapLink>(initialLinks)
        .id((d) => d.key)
        .distance(150)
    )
    .force('charge', forceManyBody().strength(-700))
    .force('center', forceCenter(width / 2, height / 2))
    .force('collide', forceCollide(48))

  simulation.on('tick', () => {
    tick.value++
  })
  simulation.on('end', fitToView)
}

function fitToView() {
  if (nodes.value.length === 0) return
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const n of nodes.value) {
    if (typeof n.x !== 'number' || typeof n.y !== 'number') continue
    minX = Math.min(minX, n.x)
    minY = Math.min(minY, n.y)
    maxX = Math.max(maxX, n.x)
    maxY = Math.max(maxY, n.y)
  }
  if (!Number.isFinite(minX)) return

  const pad = 70
  const bw = Math.max(1, maxX - minX + pad * 2)
  const bh = Math.max(1, maxY - minY + pad * 2)
  const { width, height } = size
  const k = Math.min(1.5, Math.max(0.15, Math.min(width / bw, height / bh)))
  view.k = k
  view.x = (width - (minX + maxX) * k) / 2
  view.y = (height - (minY + maxY) * k) / 2
}

watch(
  () => props.graph,
  () => {
    nextTick(() => {
      currentSize()
      build()
    })
  }
)

onMounted(() => {
  currentSize()
  build()

  resizeObserver = new ResizeObserver(() => {
    if (!props.graph) return
    currentSize()
    if (simulation) {
      simulation.force('center', forceCenter(size.width / 2, size.height / 2))
      simulation.alpha(0.3).restart()
    }
  })
  if (svgEl.value) resizeObserver.observe(svgEl.value)
})

onBeforeUnmount(() => {
  simulation?.stop()
  resizeObserver?.disconnect()
})

function onNodeClick(n: MapNode) {
  if (n.source === 'missing' && n.related) {
    emit('fetch', n.related)
  } else {
    emit('select', n.key)
  }
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const rect = svgEl.value!.getBoundingClientRect()
  const cx = e.clientX - rect.left
  const cy = e.clientY - rect.top
  const factor = e.deltaY < 0 ? 1.15 : 1 / 1.15
  const k = Math.min(5, Math.max(0.15, view.k * factor))
  view.x = cx - ((cx - view.x) * k) / view.k
  view.y = cy - ((cy - view.y) * k) / view.k
  view.k = k
}

function onPanStart(e: PointerEvent) {
  panning = true
  panStart = { x: e.clientX - view.x, y: e.clientY - view.y }
  ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
}

function onPanMove(e: PointerEvent) {
  if (!panning) return
  view.x = e.clientX - panStart.x
  view.y = e.clientY - panStart.y
}

function onPanEnd() {
  panning = false
}

let panning = false
let panStart = { x: 0, y: 0 }

function sourceClass(s: string): string {
  return 'src-' + s
}
</script>

<template>
  <div class="map-wrap">
    <svg
      ref="svgEl"
      class="map-svg"
      @wheel.prevent="onWheel"
      @pointerdown="onPanStart"
      @pointermove="onPanMove"
      @pointerup="onPanEnd"
      @pointerleave="onPanEnd"
    >
      <g :transform="`translate(${view.x},${view.y}) scale(${view.k})`">
        <line
          v-for="(l, i) in links"
          :key="'l' + i"
          class="map-link"
          :class="{ external: l.external }"
          :x1="l.source.x ?? 0"
          :y1="l.source.y ?? 0"
          :x2="l.target.x ?? 0"
          :y2="l.target.y ?? 0"
        />
        <g v-for="n in nodes" :key="n.key" class="map-node" @click.stop="onNodeClick(n)">
          <title>{{ n.type }}/{{ n.id }}</title>
          <circle :r="14" :class="sourceClass(n.source)" />
          <text class="map-label" dy="28" text-anchor="middle">{{ n.label }}</text>
        </g>
      </g>
    </svg>

    <div v-if="!graph || !graph.nodes || graph.nodes.length === 0" class="map-empty">
      Нет данных для построения карты
    </div>

    <div v-else class="map-legend">
      <span class="lg"><i class="dot src-data"></i> data</span>
      <span class="lg"><i class="dot src-included"></i> included</span>
      <span class="lg"><i class="dot src-missing"></i> нет в документе</span>
    </div>
  </div>
</template>

<style scoped>
.map-wrap {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 320px;
}

.map-svg {
  width: 100%;
  height: 100%;
  cursor: grab;
  touch-action: none;
  background:
    radial-gradient(circle at 1px 1px, var(--border) 1px, transparent 0) 0 0 / 22px 22px;
}

.map-svg:active {
  cursor: grabbing;
}

.map-link {
  stroke: var(--border-strong);
  stroke-width: 1.4;
}

.map-link.external {
  stroke: var(--orange);
  stroke-dasharray: 5 4;
}

.map-node {
  cursor: pointer;
}

.map-node circle {
  stroke: var(--bg-panel);
  stroke-width: 2.5;
}

.map-node circle.src-data {
  fill: var(--accent);
}

.map-node circle.src-included {
  fill: var(--green);
}

.map-node circle.src-missing {
  fill: var(--text-tertiary);
}

.map-node:hover circle {
  stroke: var(--orange);
  stroke-width: 3;
}

.map-label {
  font-family: var(--mono);
  font-size: 11px;
  fill: var(--text);
  paint-order: stroke;
  stroke: var(--bg-panel);
  stroke-width: 3px;
  pointer-events: none;
}

.map-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-tertiary);
  font-size: 13px;
}

.map-legend {
  position: absolute;
  left: 12px;
  bottom: 12px;
  display: flex;
  gap: 14px;
  padding: 6px 12px;
  background: var(--bg-panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: var(--shadow);
}

.lg {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-secondary);
}

.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  display: inline-block;
}

.dot.src-data {
  background: var(--accent);
}

.dot.src-included {
  background: var(--green);
}

.dot.src-missing {
  background: var(--text-tertiary);
}
</style>
