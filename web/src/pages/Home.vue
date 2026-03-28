<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
// 确保你的项目中有 Icon 组件，或者替换为实体的 SVG
import Icon from "@/components/icons/Icon.vue"

const router = useRouter()
const go = (path: string) => router.push(path)

// --- 动态监控数据逻辑 ---
const nodesCount = ref(128)
const latency = ref(42)
const lastSync = ref('14:30:00')

let timer: any
onMounted(() => {
  timer = setInterval(() => {
    latency.value = Math.floor(Math.random() * (45 - 38 + 1)) + 38
    lastSync.value = new Date().toLocaleTimeString([], { hour12: false })
  }, 3000)
})
onUnmounted(() => clearInterval(timer))

const features = [
  {
    title: 'Operator-Native CRDs',
    desc: '深度集成 Kubernetes 生态，通过声明式定义网络状态。',
    icon: 'nodes',
    tag: 'Stable',
    badgeClass: 'badge-ghost'
  },
  {
    title: 'eBPF Acceleration',
    desc: '路线图：利用 eBPF 在内核态实现数据包卸载，提升吞吐性能。',
    icon: 'cpu',
    tag: 'Roadmap',
    badgeClass: 'badge-warning badge-outline'
  },
  {
    title: 'Identity-Based Overlay',
    desc: '基于零信任身份验证建立隧道，实现资源的即时安全接入。',
    icon: 'shield',
    tag: 'Stable',
    badgeClass: 'badge-ghost'
  }
]
</script>

<template>
  <div class="min-h-screen bg-base-100 text-base-content font-sans antialiased selection:bg-primary selection:text-primary-content">

    <div class="hero min-h-[70vh] bg-base-100 relative overflow-hidden">
      <div class="absolute top-0 w-full h-full bg-primary/5 [mask-image:radial-gradient(ellipse_at_top,white,transparent)]"></div>

      <div class="hero-content text-center z-10">
        <div class="max-w-2xl">
          <div class="inline-flex items-center gap-2 px-4 py-2 mb-8 bg-base-200 border border-base-300 rounded-full text-xs font-bold tracking-widest animate-pulse">
            <span class="w-2 h-2 rounded-full bg-primary"></span>
            WIREFLOW ENGINE V0.1.2
          </div>

          <h1 class="text-4xl md:text-6xl font-black leading-tight tracking-tighter mb-8">
            构建声明式<br />
            <span class="text-primary italic">云原生智能组网</span>
          </h1>

          <p class="mb-10 text-base opacity-60 leading-relaxed font-medium">
            基于零信任架构与 Kubernetes 原生驱动，为异构多云环境提供高性能、透明且可观测的 Mesh 网络。
          </p>

          <div class="flex justify-center gap-4">
            <button class="btn btn-primary px-8 rounded-xl shadow-lg" @click="go('/dashboard')">进入控制台</button>
            <button class="btn btn-outline border-base-300 px-8 rounded-xl">架构说明</button>
          </div>
        </div>
      </div>
    </div>

    <section class="px-6 -mt-10 mb-24 relative z-20">
      <div class="max-w-5xl mx-auto">
        <div class="mockup-window border border-base-300 bg-base-200 shadow-2xl overflow-hidden">
          <div class="bg-base-100 border-t border-base-300">
            <div class="stats stats-vertical lg:stats-horizontal w-full rounded-none bg-transparent">
              <div class="stat p-10">
                <div class="stat-title uppercase text-[10px] font-black tracking-widest opacity-40">Active Nodes</div>
                <div class="stat-value text-4xl font-mono">{{ nodesCount }}</div>
                <div class="stat-desc text-success mt-1 font-bold">● FABRIC ONLINE</div>
              </div>

              <div class="stat p-10">
                <div class="stat-title uppercase text-[10px] font-black tracking-widest opacity-40">Avg Latency</div>
                <div class="stat-value text-primary text-4xl font-mono">{{ latency }}ms</div>
                <div class="stat-desc mt-1 font-medium italic opacity-40">Sync: {{ lastSync }}</div>
              </div>

              <div class="stat p-10">
                <div class="stat-title uppercase text-[10px] font-black tracking-widest opacity-40">Data Plane</div>
                <div class="stat-value text-secondary text-4xl font-black italic">eBPF</div>
                <div class="stat-desc mt-1">
                  <span class="badge badge-warning badge-xs font-bold uppercase">Roadmap</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="py-24 bg-base-200/50 border-y border-base-300">
      <div class="max-w-6xl mx-auto px-6">
        <div class="grid grid-cols-1 md:grid-cols-3 gap-8">
          <div v-for="feat in features" :key="feat.title" class="card bg-base-100 shadow-sm border border-base-300 hover:shadow-md transition-all duration-300">
            <div class="card-body p-8 gap-4">
              <div class="w-12 h-12 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
                <Icon :name="feat.icon" class="w-6 h-6" />
              </div>
              <div>
                <div :class="['badge mb-3 font-bold text-[10px] uppercase tracking-tighter', feat.badgeClass]">
                  {{ feat.tag }}
                </div>
                <h3 class="text-xl font-bold mb-2 tracking-tight">{{ feat.title }}</h3>
                <p class="text-sm opacity-50 leading-relaxed">{{ feat.desc }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>


    <section class="py-20 px-6">
      <div class="max-w-5xl mx-auto">
        <div class="card lg:card-side bg-neutral text-neutral-content shadow-2xl overflow-hidden border border-white/5">

          <div class="card-body lg:w-1/3 p-8 md:p-10 justify-center border-b lg:border-b-0 lg:border-r border-white/5">
            <div class="flex items-center gap-2 mb-4">
              <div class="badge badge-primary badge-outline badge-sm font-black uppercase tracking-tighter">IaC</div>
            </div>
            <h2 class="text-2xl font-black italic tracking-tight mb-2">Infrastructure<br/>as Code</h2>
            <p class="text-xs opacity-50 leading-relaxed max-w-xs">
              通过声明式指令实现节点自动发现与握手，让复杂的组网逻辑简化为一行部署命令。
            </p>
          </div>

          <div class="lg:w-2/3 bg-black/40 p-6 md:p-8 flex items-center">
            <div class="mockup-code w-full bg-transparent border-none text-[13px] font-mono leading-relaxed">
              <pre data-prefix="λ" class="text-primary/60 italic"><code># Standard Deployment</code></pre>
              <pre data-prefix="$"><code>docker run -d --privileged \</code></pre>
              <pre data-prefix=">"><code>  -e CONTROLLER="https://api.wireflow.io" \</code></pre>
              <pre data-prefix=">"><code>  ghcr.io/wireflow/edge:stable</code></pre>
            </div>
          </div>

        </div>
      </div>
    </section>

    <footer class="footer footer-center p-10 bg-base-100 text-base-content/20">
      <div class="text-[10px] font-black uppercase tracking-[0.5em]">
        © 2026 Wireflow SDN Fabric · Powered by DaisyUI
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* 仅保留最基础的动画，避免 CSS 报错 */
.animate-pulse {
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: .5; }
}
</style>