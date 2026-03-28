<script setup>
import { ref } from 'vue'

const records = ref([
  { name: 'api.wireflow.local', type: 'A', value: '10.0.0.2', ttl: '3600', status: 'Active' },
  { name: 'db.wireflow.local', type: 'A', value: '10.0.0.3', ttl: '60', status: 'Active' },
  { name: 'dashboard.wireflow.io', type: 'CNAME', value: 'lb-prod.wireflow.io', ttl: '300', status: 'Active' },
])

const loading = ref(false)
const searchQuery = ref('')

const getTypeClass = (type) => {
  const map = {
    'A': 'bg-blue-100 text-blue-700 border-blue-200',
    'CNAME': 'bg-purple-100 text-purple-700 border-purple-200',
    'TXT': 'bg-amber-100 text-amber-700 border-amber-200',
    'AAAA': 'bg-emerald-100 text-emerald-700 border-emerald-200'
  }
  return map[type] || 'bg-gray-100 text-gray-700'
}
</script>

<template>
  <div class="max-w-7xl mx-auto p-4 lg:p-8 space-y-6">

    <div class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-base-300 pb-6">
      <div>
        <h1 class="text-3xl font-extrabold tracking-tight flex items-center gap-3">
          <div class="p-2 bg-secondary/10 rounded-xl text-secondary">
            <svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9-9c1.657 0 3 4.03 3 9s-1.343 9-3 9m0-18c-1.657 0-3 4.03-3 9s1.343 9 3 9m-9-9a9 9 0 019-9" />
            </svg>
          </div>
          内网 DNS 解析
        </h1>
        <p class="text-sm opacity-50 mt-2 max-w-xl">
          管理 WireFlow 私有网络的域名解析记录。支持 A、CNAME 等多种记录类型，实现服务发现自动化。
        </p>
      </div>
      <button class="btn btn-primary shadow-lg shadow-primary/20 rounded-xl px-8">+ 新增解析记录</button>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase">记录总数</div>
          <div class="stat-value text-2xl font-mono">{{ records.length }}</div>
          <div class="stat-desc mt-1 italic">Active Records</div>
        </div>
      </div>
      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase text-info">主要解析 (A)</div>
          <div class="stat-value text-2xl font-mono text-info">
            {{ records.filter(r => r.type === 'A').length }}
          </div>
          <div class="stat-desc uppercase text-[9px]">Endpoint mapping</div>
        </div>
      </div>
      <div class="stats shadow-sm border border-base-300 bg-base-100 hidden lg:flex">
        <div class="stat">
          <div class="stat-title text-[10px] font-bold uppercase">查询 QPS</div>
          <div class="stat-value text-2xl font-mono">124</div>
          <div class="stat-desc font-mono uppercase text-[9px] text-success">DNS Cluster Healthy</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 rounded-2xl border border-base-300 shadow-sm overflow-hidden">
      <div class="p-4 bg-base-200/30 border-b border-base-300 flex flex-col md:flex-row items-center gap-4">
        <div class="relative flex-1 w-full md:max-w-md">
          <svg class="w-4 h-4 absolute left-3 top-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
          <input v-model="searchQuery" class="input input-bordered input-sm w-full pl-10 bg-base-100" placeholder="搜索域名或解析值..." />
        </div>
        <div class="flex items-center gap-2">
          <span class="text-[10px] font-bold opacity-30 uppercase">Filter:</span>
          <div class="flex gap-1">
            <button v-for="t in ['A', 'CNAME', 'TXT']" :key="t" class="btn btn-xs btn-ghost border border-base-300">{{ t }}</button>
          </div>
        </div>
      </div>

      <div class="divide-y divide-base-300">
        <div v-for="r in records" :key="r.name"
             class="group flex flex-col md:flex-row items-center justify-between gap-4 p-4 px-6 hover:bg-base-200/50 transition-colors">

          <div class="flex items-center gap-4 w-full md:w-1/3">
            <div :class="['w-12 h-6 flex items-center justify-center text-[10px] font-bold rounded border shrink-0', getTypeClass(r.type)]">
              {{ r.type }}
            </div>
            <div class="truncate">
              <div class="font-bold text-sm group-hover:text-primary transition-colors cursor-pointer">{{ r.name }}</div>
              <div class="text-[10px] opacity-40 font-mono tracking-tight">TTL: {{ r.ttl }}s</div>
            </div>
          </div>

          <div class="flex-1 flex items-center gap-2 overflow-hidden">
            <svg class="w-3 h-3 opacity-20 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" /></svg>
            <span class="text-xs font-mono opacity-60 truncate bg-base-200/50 px-2 py-1 rounded">
                {{ r.value }}
             </span>
          </div>

          <div class="flex items-center gap-2 shrink-0 w-full md:w-auto justify-end">
            <button class="btn btn-ghost btn-sm text-xs font-bold hover:bg-base-200">修改</button>
            <div class="w-px h-4 bg-base-300 mx-1"></div>
            <button class="btn btn-ghost btn-sm text-error/40 hover:text-error hover:bg-error/10">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="alert bg-base-200/50 border-base-300 text-[11px] opacity-60">
      <svg class="w-4 h-4 text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
      <span>所有解析记录将同步至 CoreDNS 集群。修改 TTL 后，客户端生效可能存在延迟。</span>
    </div>
  </div>
</template>