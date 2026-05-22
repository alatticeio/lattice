<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

interface SimNode {
  id: number
  x: number
  y: number
  r: number
  vx: number
  vy: number
}

interface Link {
  source: number
  target: number
}

const props = withDefaults(defineProps<{
  nodeCount?: number
  animated?: boolean
}>(), {
  nodeCount: 12,
  animated: true,
})

const svgRef = ref<SVGSVGElement>()
const nodes = ref<SimNode[]>([])
const links = ref<Link[]>([])
let raf = 0

function initGraph(w: number, h: number) {
  const ns: SimNode[] = []
  const ls: Link[] = []

  for (let i = 0; i < props.nodeCount; i++) {
    ns.push({
      id: i,
      x: Math.random() * w,
      y: Math.random() * h,
      r: 3 + Math.random() * 4,
      vx: (Math.random() - 0.5) * 0.3,
      vy: (Math.random() - 0.5) * 0.3,
    })
  }

  for (let i = 0; i < ns.length; i++) {
    const j = (i + 1) % ns.length
    ls.push({ source: i, target: j })
    if (Math.random() > 0.5 && i + 2 < ns.length) {
      ls.push({ source: i, target: i + 2 })
    }
  }

  nodes.value = ns
  links.value = ls
}

function tick() {
  if (!props.animated) return
  const ns = nodes.value
  const margin = 40
  const w = svgRef.value?.clientWidth ?? 800
  const h = svgRef.value?.clientHeight ?? 600

  for (const n of ns) {
    n.x += n.vx
    n.y += n.vy
    if (n.x < margin || n.x > w - margin) n.vx *= -1
    if (n.y < margin || n.y > h - margin) n.vy *= -1
  }
  raf = requestAnimationFrame(tick)
}

onMounted(() => {
  const w = svgRef.value?.clientWidth ?? 800
  const h = svgRef.value?.clientHeight ?? 600
  initGraph(w, h)
  if (props.animated) raf = requestAnimationFrame(tick)
})

onUnmounted(() => cancelAnimationFrame(raf))
</script>

<template>
  <svg ref="svgRef" class="w-full h-full" viewBox="0 0 800 600" preserveAspectRatio="xMidYMid slice">
    <line
      v-for="(l, i) in links"
      :key="'l' + i"
      :x1="nodes[l.source]?.x ?? 0"
      :y1="nodes[l.source]?.y ?? 0"
      :x2="nodes[l.target]?.x ?? 0"
      :y2="nodes[l.target]?.y ?? 0"
      class="lattice-topology-line"
    />
    <circle
      v-for="n in nodes"
      :key="n.id"
      :cx="n.x"
      :cy="n.y"
      :r="n.r"
      class="lattice-topology-node"
    >
      <animate
        attributeName="r"
        :values="`${n.r};${n.r + 1.5};${n.r}`"
        :dur="2 + n.id * 0.3"
        repeatCount="indefinite"
      />
    </circle>
  </svg>
</template>
