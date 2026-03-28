<script setup lang="ts">
import { ref, computed } from 'vue'
import Icon from '../components/icons/Icon.vue'

// 1. 模拟桥接数据
const bridges = ref([
  {
    id: 'br-001',
    name: '上海-北京跨区链',
    from: { name: '上海生产环境', id: 'ws-101', color: 'bg-primary' },
    to: { name: '北京研发中心', id: 'ws-102', color: 'bg-indigo-500' },
    status: 'active',
    type: 'Bi-Directional', // 双向
    traffic: '1.2 GB/s',
    latency: '32ms',
    createdAt: '2025-12-01'
  },
  {
    id: 'br-002',
    name: '广州备份同步',
    from: { name: '广州分拣区', id: 'ws-103', color: 'bg-emerald-500' },
    to: { name: '总部核心库', id: 'ws-000', color: 'bg-slate-700' },
    status: 'warning',
    type: 'One-Way', // 单向
    traffic: '450 MB/s',
    latency: '58ms',
    createdAt: '2026-01-10'
  }
])

const showCreateModal = ref(false)
</script>

<template>
  <div class="max-w-[1400px] mx-auto p-4 lg:p-6 space-y-6 animate-fade-in">

    <div class="flex flex-col md:flex-row justify-between items-end gap-4 border-b border-slate-200 dark:border-white/5 pb-6">
      <div>
        <h1 class="text-2xl font-black tracking-tighter text-base-content dark:text-white flex items-center gap-2">
          对等连接中心
          <div class="flex items-center -space-x-2 ml-1">
            <div class="w-3 h-3 rounded-full bg-primary/20 border border-blue-500/50"></div>
            <div class="w-3 h-3 rounded-full bg-indigo-500/20 border border-indigo-500/50"></div>
          </div>
        </h1>
        <p class="text-[11px] font-medium text-base-content/60 dark:text-base-content/40 mt-1">
          管理跨工作空间 (Cross-Workspace) 的网络穿透与受控对等通信。
        </p>
      </div>
      <button @click="showCreateModal = true" class="btn btn-primary btn-sm rounded-xl px-6 font-black tracking-widest shadow-lg shadow-primary/20">
        <Icon name="plus" class="w-3.5 h-3.5 mr-1" />
        建立新对等
      </button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div v-for="item in [
        { l: '活跃对等链', v: '08', i: 'link' },
        { l: '跨区总流量', v: '4.8 TB', i: 'chart' },
        { l: '平均时延', v: '24ms', i: 'clock' }
      ]" :key="item.l" class="bg-base-100 dark:bg-slate-900/50 p-4 rounded-2xl border border-slate-200/60 flex items-center gap-4">
        <div class="w-10 h-10 rounded-xl bg-slate-50 dark:bg-base-100/5 flex items-center justify-center text-base-content/40">
          <Icon :name="item.i" class="w-5 h-5" />
        </div>
        <div>
          <div class="text-[9px] font-black text-base-content/40 uppercase tracking-widest">{{ item.l }}</div>
          <div class="text-lg font-black text-slate-700 dark:text-slate-200">{{ item.v }}</div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-[1fr_300px] gap-6">
      <div class="space-y-4">
        <div v-for="br in bridges" :key="br.id"
             class="group bg-base-100 dark:bg-slate-900/50 border border-slate-200/60 dark:border-white/5 rounded-[1.5rem] p-5 hover:shadow-xl hover:shadow-indigo-500/5 transition-all duration-500"
        >
          <div class="flex flex-col lg:flex-row items-center gap-6">
            <div class="flex items-center gap-3 w-full lg:w-48 shrink-0">
              <div :class="['w-10 h-10 rounded-xl flex items-center justify-center text-white font-black text-sm shadow-inner', br.from.color]">
                {{ br.from.name[0] }}
              </div>
              <div class="min-w-0">
                <div class="text-[9px] font-black text-base-content/40 uppercase">Source</div>
                <div class="text-[13px] font-bold text-slate-700 dark:text-slate-200 truncate">{{ br.from.name }}</div>
              </div>
            </div>

            <div class="flex-1 flex flex-col items-center w-full">
              <div class="w-full flex items-center gap-2">
                <div class="h-[1px] flex-1 bg-slate-100"></div>

                <div class="flex items-center gap-4 px-4 py-1.5 bg-slate-50 dark:bg-base-100/5 rounded-full border border-slate-100 dark:border-white/5 relative group-hover:border-indigo-500/30 transition-colors">
                  <Icon v-if="br.type === 'Bi-Directional'" name="chevron-left" class="w-3 h-3 text-indigo-400" />
                  <div class="flex flex-col items-center min-w-[80px]">
                    <span class="text-[9px] font-black text-indigo-500 uppercase tracking-[0.2em] italic">{{ br.type }}</span>
                    <div class="flex items-center gap-1 mt-0.5">
                      <span class="text-[10px] font-mono text-base-content/40">{{ br.latency }}</span>
                    </div>
                  </div>
                  <Icon name="chevron-right" class="w-3 h-3 text-indigo-400" />

                  <div class="absolute inset-0 rounded-full bg-indigo-500/5 animate-pulse"></div>
                </div>

                <div class="h-[1px] flex-1 bg-slate-100"></div>
              </div>
            </div>

            <div class="flex items-center gap-3 w-full lg:w-48 shrink-0 lg:justify-end text-right">
              <div class="min-w-0">
                <div class="text-[9px] font-black text-base-content/40 uppercase">Destination</div>
                <div class="text-[13px] font-bold text-slate-700 dark:text-slate-200 truncate">{{ br.to.name }}</div>
              </div>
              <div :class="['w-10 h-10 rounded-xl flex items-center justify-center text-white font-black text-sm shadow-inner', br.to.color]">
                {{ br.to.name[0] }}
              </div>
            </div>

            <div class="flex items-center gap-2 pl-4 border-l border-slate-100 dark:border-white/5">
              <button class="btn btn-ghost btn-xs btn-square opacity-40 hover:opacity-100"><Icon name="edit" /></button>
              <button class="btn btn-ghost btn-xs btn-square text-error/50 hover:text-error hover:bg-error/10"><Icon name="trash" /></button>
            </div>
          </div>
        </div>
      </div>

      <aside class="space-y-4">
        <div class="bg-indigo-600 rounded-2xl p-6 text-white shadow-xl shadow-indigo-600/20">
          <h3 class="text-sm font-black italic">什么是对等连接？</h3>
          <p class="text-[10px] text-indigo-100 mt-3 leading-relaxed opacity-80">
            默认情况下，工作空间是物理隔离的。对等连接允许你在两个空间之间建立基于 Wireguard 的加密隧道，实现跨 Namespace 的服务直接访问。
          </p>
          <div class="mt-4 pt-4 border-t border-white/10 space-y-2">
            <div class="flex items-center gap-2 text-[9px] font-bold uppercase">
              <Icon name="check" class="w-3 h-3" /> 二层内网互通
            </div>
            <div class="flex items-center gap-2 text-[9px] font-bold uppercase">
              <Icon name="check" class="w-3 h-3" /> 自动路由下发
            </div>
          </div>
        </div>

        <div class="bg-base-100 dark:bg-slate-900/50 border border-slate-200/60 rounded-2xl p-5">
          <h4 class="text-[10px] font-black uppercase text-base-content/40 mb-4">连接健康度说明</h4>
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold text-base-content/70">双向对等 (Bi)</span>
              <span class="badge badge-xs bg-indigo-500 border-none text-white font-bold">全互通</span>
            </div>
            <div class="flex items-center justify-between">
              <span class="text-[11px] font-bold text-base-content/70">单向出口 (One)</span>
              <span class="badge badge-xs bg-slate-200 border-none text-base-content/60 font-bold">受控访问</span>
            </div>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>