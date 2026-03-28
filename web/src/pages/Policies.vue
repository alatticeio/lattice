<script setup lang="ts">
import {inject} from 'vue'
import SideDrawer from '@/components/SideDrawer.vue'
import Pagination from '@/components/Pagination.vue'
// 假设你已经创建了这些组件
import {useThemeStore} from "@/stores/useTheme";
import {usePolicyPageStore} from "@/stores/usePolicyPageStore";
import Icon from "@/components/icons/Icon.vue";

const themeStore = useThemeStore()

const policyStore = usePolicyPageStore()
const toast = inject('globalToast')

</script>
<template>
  <div class="max-w-7xl mx-auto p-4 lg:p-8 space-y-6">

    <div
        class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-base-300 pb-6">
      <div>
        <h1 class="text-3xl font-extrabold tracking-tight flex items-center gap-3">
          <div class="p-2 bg-primary/10 rounded-xl">
            <svg class="w-8 h-8 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/>
            </svg>
          </div>
          策略管理中心
        </h1>
        <p class="text-sm opacity-50 mt-2 max-w-xl leading-relaxed">
          此处负责定义集群内的网络隔离与访问控制。通过配置 **NetworkPolicy**，您可以精确控制 Pod 之间的流量走向，构建零信任安全架构。
        </p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-ghost border-base-300 rounded-xl" @click="policyStore.actions.refresh">
          <svg :class="['w-4 h-4', policyStore.loading ? 'animate-spin' : '']" fill="none" stroke="currentColor"
               viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
        </button>
        <button class="btn btn-primary btn-md shadow-lg shadow-primary/20 rounded-xl px-8"
                @click="policyStore.actions.openDrawer('create')">
          + 新建策略
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-xs font-bold uppercase">已部署策略</div>
          <div class="stat-value text-2xl font-mono">{{ policyStore.rows.length }}</div>
          <div class="stat-desc font-mono mt-1">Active Rules</div>
        </div>
      </div>
      <div class="stats shadow-sm border border-base-300 bg-base-100">
        <div class="stat">
          <div class="stat-title text-xs font-bold uppercase text-success">允许访问 (Allow)</div>
          <div class="stat-value text-2xl font-mono text-success">
            {{ policyStore.rows.filter((p:any) => p.mode === 'allow').length }}
          </div>
          <div class="stat-desc">Whitelist mode</div>
        </div>
      </div>
    </div>

    <div class="bg-base-100 rounded-2xl border border-base-300 shadow-sm overflow-hidden">
      <div class="p-4 bg-base-200/30 border-b border-base-300 flex items-center gap-4">
        <div class="relative flex-1 w-full md:max-w-md">
          <svg class="w-4 h-4 absolute left-3 top-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
          </svg>
          <input v-model="policyStore.params.search"
                 @keyup.enter="policyStore.actions.refresh"
                 class="input input-bordered input-sm w-full pl-10 bg-base-100 focus:border-primary"
                 placeholder="搜索节点名称、IP 地址..."/>
          <button
              @click="policyStore.actions.refresh"
              class="absolute right-2 top-1.5 btn btn-ghost btn-xs opacity-50 hover:opacity-100"
          >
            Enter
          </button>
        </div>
        <div class="hidden sm:flex gap-2 text-[10px] font-bold opacity-40 uppercase">
          Sorted by: Updated At
        </div>
      </div>

      <div class="divide-y divide-base-300">
        <div v-for="r in policyStore.rows" :key="r.name"
             class="group flex flex-col md:flex-row items-center justify-between gap-4 p-4 px-6 hover:bg-base-200/50 transition-colors">

          <div class="flex items-center gap-4 w-full md:w-1/3">
            <div :class="[r.mode === 'allow' ? 'bg-success' : 'bg-error', 'w-2 h-2 rounded-full']"></div>
            <div
                :class="[
    'w-6 h-6 flex items-center justify-center rounded-md text-[10px] font-bold text-white shadow-sm transition-transform group-hover:scale-110',
    themeStore.actions.getAvatarColor(r.name)
  ]"
            >
              {{ themeStore.actions.getFirstChar(r.name) }}
            </div>
            <div>
              <div class="font-bold text-sm group-hover:text-primary transition-colors cursor-pointer"
                   @click="policyStore.actions.openDrawer('view', r)">{{ r.name }}
              </div>
              <div class="text-[10px] opacity-40 font-mono">{{ r.updatedAt }}</div>
            </div>
          </div>

          <div class="hidden md:block flex-1 text-xs opacity-50 truncate">
            {{ r.description || '此策略暂无详细备注说明。' }}
          </div>

          <div class="flex items-center gap-2 shrink-0">
            <button class="btn btn-ghost btn-sm text-xs font-bold hover:bg-base-300" @click="policyStore.actions.openDrawer('view', r)">
              详情
            </button>
            <div class="w-px h-4 bg-base-300 mx-1"></div>
            <button class="btn btn-ghost btn-xs text-error/90 hover:text-error hover:bg-error/40" @click="policyStore.actions.handleDelete(r, toast)">
              <Icon name="trash" class="w-4 h-4"/>
            </button>
          </div>


        </div>

        <!-- 分页部分 -->
        <Pagination
            v-model:page="policyStore.params.page"
            v-model:pageSize="policyStore.params.pageSize"
            :total="policyStore.total"
            item-name="策略"
        />

        <div v-if="policyStore.rows.length === 0" class="p-20 text-center opacity-30 flex flex-col items-center">
          <svg class="w-12 h-12 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
                d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
          </svg>
          <p class="text-sm font-bold">未找到匹配的策略</p>
        </div>

      </div>
    </div>

    <SideDrawer
        v-model="policyStore.ui.isDrawerOpen"
        :title="policyStore.ui.drawerType === 'view' ? '策略详情' : (policyStore.ui.drawerType === 'create' ? '新建安全策略' : '编辑策略')"
        subtitle="Network Security Orchestration"
        :level="policyStore.ui.drawerType === 'view' ? 'md' : 'xl'"
    >
      <template #icon>
        <div class="p-2 bg-primary rounded-lg text-white">
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
          </svg>
        </div>
      </template>

      <div v-if="policyStore.ui.drawerType === 'view'" class="space-y-6">
        <div class="space-y-8">
          <div class="relative p-6 rounded-2xl bg-primary/5 border border-primary/20 overflow-hidden">
            <div class="absolute right-[-20px] top-[-20px] opacity-10">
              <svg class="w-32 h-32 text-primary" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5-10-5-10 5z"/>
              </svg>
            </div>
            <div class="relative z-10">
              <span class="text-[10px] font-bold text-primary uppercase tracking-widest">受保护的目标 / Target</span>
              <h3 class="text-2xl font-black mt-1">{{ policyStore.activePolicy.name }}</h3>
              <div class="flex gap-2 mt-3">
                <div class="badge badge-primary font-mono px-3 py-3">app:
                  {{ policyStore.activePolicy.peerSelector?.matchLabels?.app || '所有 节点' }}
                </div>
              </div>
              <p class="text-xs opacity-50 mt-4 leading-relaxed">
                此策略将应用于集群中带有上述标签的所有 Pod，控制它们的网络进出权限。
              </p>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-6">

            <div class="collapse collapse-arrow bg-base-200/50 border border-base-300 rounded-2xl">
              <input type="checkbox" checked/>
              <div class="collapse-title flex items-center gap-4">
                <div class="p-2 bg-secondary/10 text-secondary rounded-lg">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                          d="M11 16l-4-4m0 0l4-4m-4 4h14"/>
                  </svg>
                </div>
                <div>
                  <h4 class="font-bold">入站白名单 (Ingress)</h4>
                  <p class="text-[10px] opacity-50 uppercase font-mono">Who can access this service</p>
                </div>
              </div>
              <div class="collapse-content space-y-3">
                <div v-if="!policyStore.activePolicy.ingress?.length" class="alert bg-error/5 text-error text-xs border-error/10">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
                  </svg>
                  <span>全隔离模式：拒绝所有外部连接直达此服务。</span>
                </div>
                <div v-for="rule in policyStore.activePolicy.ingress"
                     class="flex items-center gap-4 p-3 bg-base-100 rounded-xl border border-base-300 shadow-sm">
                  <div class="flex-1">
                    <div class="text-[10px] opacity-40 font-bold uppercase">允许来自</div>
                    <div class="text-sm font-mono font-bold text-secondary">role:
                      {{ rule._rawLables || 'Any' }}
                    </div>
                  </div>
                  <div class="divider divider-horizontal mx-0 opacity-20"></div>
                  <div class="w-24">
                    <div class="text-[10px] opacity-40 font-bold uppercase">通过端口</div>
                    <div class="text-sm font-mono font-bold">{{ rule.ports[0]?.port || '所有' }}</div>
                  </div>
                </div>
              </div>
            </div>

            <div class="collapse collapse-arrow bg-base-200/50 border border-base-300 rounded-2xl">
              <input type="checkbox" checked/>
              <div class="collapse-title flex items-center gap-4">
                <div class="p-2 bg-accent/10 text-accent rounded-lg">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l4 4m0 0l-4 4m4-4H3"/>
                  </svg>
                </div>
                <div>
                  <h4 class="font-bold">出站白名单 (Egress)</h4>
                  <p class="text-[10px] opacity-50 uppercase font-mono">What this service can access</p>
                </div>
              </div>
              <div class="collapse-content space-y-3">
                <div v-if="!policyStore.activePolicy.egress?.length" class="alert bg-error/5 text-error text-xs border-error/10">
                  <span>全隔离模式：此服务禁止主动发起任何外部请求。</span>
                </div>
                <div v-for="rule in policyStore.activePolicy.egress"
                     class="flex items-center gap-4 p-3 bg-base-100 rounded-xl border border-base-300 shadow-sm">
                  <div class="flex-1">
                    <div class="text-[10px] opacity-40 font-bold uppercase">允许访问</div>
                    <div class="text-sm font-mono font-bold text-accent">role:
                      {{ rule._rawLabels || 'Any' }}
                    </div>
                  </div>
                  <div class="divider divider-horizontal mx-0 opacity-20"></div>
                  <div class="w-24">
                    <div class="text-[10px] opacity-40 font-bold uppercase">目的端口</div>
                    <div class="text-sm font-mono font-bold">{{ rule.ports[0]?.port || '所有' }}</div>
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>
      </div>

      <div v-else class="space-y-6">

        <div class="flex flex-1 overflow-hidden">
          <div class="flex-1 overflow-y-auto p-6 space-y-8 custom-scrollbar">

            <section>
              <h4 class="text-[10px] font-bold opacity-40 mb-3 uppercase tracking-tighter">Quick Templates /
                场景快选</h4>
              <div class="flex flex-wrap gap-2">
                <button @click="policyStore.actions.applyTemplate('isolate')" class="btn btn-xs btn-outline rounded-full">🚫 全隔离</button>
                <button @click="policyStore.actions.applyTemplate('db')" class="btn btn-xs btn-outline rounded-full">🔐 数据库保护</button>
                <button @click="policyStore.actions.applyTemplate('internet')" class="btn btn-xs btn-outline rounded-full">🌐 允许出站
                </button>
              </div>
            </section>

            <div class="flex flex-wrap md:flex-nowrap gap-4 items-start">
              <div class="relative flex-1 min-w-[200px]">
                <span class="absolute -top-2.5 left-3 px-2 bg-base-100 text-[10px] font-black text-primary uppercase tracking-tighter rounded border border-base-300 z-10">策略名称</span>
                <input v-model="policyStore.form.name" class="input input-sm input-bordered w-full font-mono" />
              </div>

              <div class="relative w-48">
                <span class="absolute -top-2.5 left-3 px-2 bg-base-100 text-[10px] font-black text-primary uppercase tracking-tighter rounded border border-base-300 z-10">应用范围</span>
                <input v-model="policyStore.form._targetLabel" class="input input-sm input-bordered w-full font-mono" />
              </div>

              <div class="relative w-32">
                <span class="absolute -top-2.5 left-3 px-2 bg-base-100 text-[10px] font-black text-primary uppercase tracking-tighter rounded border border-base-300 z-10">类型</span>
                <select v-model="policyStore.form.action" class="select select-sm select-bordered w-full font-mono">
                  <option>Allow</option><option>Deny</option>
                </select>
              </div>
            </div>

            <div class="relative">
              <span class="absolute -top-2.5 left-3 px-2 bg-base-100 text-[10px] font-black text-primary uppercase tracking-tighter rounded border border-base-300 z-10">描述备注</span>
              <input v-model="policyStore.form.description"
                     class="input input-sm input-bordered w-full italic text-xs"
                     placeholder="简单描述此策略，例如：控制生产环境 DB 访问..."/>
            </div>

            <div class="form-control">
              <!--              <label class="label"><span class="label-text font-bold opacity-70">生效方向 (Policy Types)</span></label>-->
              <label class="label pb-1">
                <span class="text-[11px] font-black opacity-50 uppercase tracking-wider">生效方向 / Policy Types</span>
              </label>
              <div class="flex gap-2">
                <button
                    v-for="t in ['Ingress', 'Egress']"
                    @click="policyStore.form.policyTypes.includes(t) ? policyStore.form.policyTypes = policyStore.form.policyTypes.filter(i => i !== t) : policyStore.form.policyTypes.push(t)"
                    :class="['btn btn-sm grow rounded-xl', policyStore.form.policyTypes.includes(t) ? (t === 'Ingress' ? 'btn-secondary' : 'btn-accent') : 'btn-ghost bg-base-200']"
                >
                  {{ t }}
                </button>
              </div>
            </div>

            <div v-if="policyStore.form.policyTypes.includes('Ingress')" class="space-y-4">
              <div class="flex justify-between items-center border-b border-secondary/20 pb-2">
                <h4 class="text-sm font-bold text-secondary flex items-center gap-2">
                  <div class="w-2 h-2 rounded-full bg-secondary"></div>
                  Ingress 入站规则
                </h4>
                <button @click="policyStore.actions.addRule('ingress')" class="btn btn-ghost btn-xs text-secondary hover:bg-secondary/10">+
                  添加
                </button>
              </div>
              <div v-if="policyStore.form.ingress?.length === 0"
                   class="text-center p-6 bg-base-200/30 rounded-2xl border-2 border-dashed border-base-300 opacity-40 italic text-xs">
                隔离模式：拒绝所有入站流量
              </div>
              <div v-for="(rule, idx) in policyStore.form.ingress" :key="'in-'+idx"
                   class="card bg-base-200/50 border border-base-300 relative overflow-hidden transition-all hover:border-secondary/40">
                <div class="absolute left-0 top-0 bottom-0 w-1 bg-secondary"></div>
                <div class="card-body p-4 grid grid-cols-2 gap-4">
                  <div class="form-control">
                    <span class="text-[9px] uppercase font-bold opacity-40 mb-1">From Role Label</span>
                    <input v-model="rule._rawLabel"
                           class="input input-sm input-bordered font-mono" placeholder="app=backend"/>
                  </div>

                  <div class="form-control">
                    <span class="text-[9px] uppercase font-bold opacity-40 mb-1">Protocol & Port</span>
                    <div class="flex gap-2">
                      <select v-model="rule.ports[0].protocol"
                              class="select select-sm select-bordered font-mono text-xs">
                        <option value="TCP">TCP</option>
                        <option value="UDP">UDP</option>
                      </select>

                      <input
                          v-model.number="rule.ports[0].port"
                          type="number"
                          class="input input-sm input-bordered font-mono grow"
                          placeholder="80"
                      />
                      <button @click="policyStore.form.ingress.splice(idx, 1)" class="btn btn-ghost btn-xs text-error">✕</button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="policyStore.form.policyTypes.includes('Egress')" class="space-y-4 pt-4">
              <div class="flex justify-between items-center border-b border-accent/20 pb-2">
                <h4 class="text-sm font-bold text-accent flex items-center gap-2">
                  <div class="w-2 h-2 rounded-full bg-accent"></div>
                  Egress 出站规则
                </h4>
                <button @click="policyStore.actions.addRule('egress')" class="btn btn-ghost btn-xs text-accent hover:bg-accent/10">+ 添加
                </button>
              </div>
              <div v-if="policyStore.form.egress?.length === 0"
                   class="text-center p-6 bg-base-200/30 rounded-2xl border-2 border-dashed border-base-300 opacity-40 italic text-xs">
                隔离模式：拒绝所有出站流量
              </div>
              <div v-for="(rule, idx) in policyStore.form.egress" :key="'eg-'+idx"
                   class="card bg-base-200/50 border border-base-300 relative overflow-hidden transition-all hover:border-accent/40">
                <div class="absolute left-0 top-0 bottom-0 w-1 bg-accent"></div>
                <div class="card-body p-4 grid grid-cols-2 gap-4">
                  <div class="form-control">
                    <span class="text-[9px] uppercase font-bold opacity-40 mb-1">To Role Label</span>
                    <input v-model="rule._rawLabel"
                           class="input input-sm input-bordered font-mono" placeholder="database"/>
                  </div>
                  <div class="form-control">


                    <span class="text-[9px] uppercase font-bold opacity-40 mb-1">Protocol & Port</span>
                    <div class="flex gap-2">
                      <select v-model="rule.ports[0].protocol"
                              class="select select-sm select-bordered font-mono text-xs">
                        <option value="TCP">TCP</option>
                        <option value="UDP">UDP</option>
                      </select>

                      <input
                          v-model.number="rule.ports[0].port"
                          type="number"
                          class="input input-sm input-bordered font-mono grow"
                          placeholder="80"
                      />
                      <button @click="policyStore.form.egress.splice(idx, 1)" class="btn btn-ghost btn-xs text-error">✕</button>
                    </div>

                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="w-120 bg-neutral flex flex-col border-l border-base-300 hidden lg:flex shadow-2xl">
            <div
                class="px-4 py-3 bg-black/30 text-[10px] font-bold text-base-content/40 tracking-widest flex justify-between items-center border-b border-white/5">
              <span>LIVE YAML PREVIEW</span>
              <div class="flex gap-1">
                <div class="w-2 h-2 rounded-full bg-red-500/50"></div>
                <div class="w-2 h-2 rounded-full bg-yellow-500/50"></div>
                <div class="w-2 h-2 rounded-full bg-green-500/50"></div>
              </div>
            </div>
            <div
                class="flex-1 p-5 font-mono text-[11px] text-emerald-400/90 leading-relaxed overflow-auto scrollbar-hide select-all whitespace-pre">
              {{ policyStore.yamlPreview }}
            </div>
          </div>
        </div>

      </div>

      <template #footer>
        <div class="flex gap-3">
          <button v-if="policyStore.ui.drawerType === 'view'" class="btn btn-primary flex-1" @click="policyStore.ui.drawerType = 'edit'">
            编辑
          </button>

          <button v-else-if="policyStore.ui.drawerType === 'create'" class="btn btn-primary flex-1" @click="policyStore.actions.handleCreateOrUpdate(toast)">
            立即创建
          </button>

          <button v-else class="btn btn-primary flex-1" @click="policyStore.actions.handleCreateOrUpdate(toast)">
            保存更新
          </button>

          <button class="btn btn-ghost border-base-300 flex-1" @click="policyStore.ui.isDrawerOpen = false">
            取消
          </button>
        </div>
      </template>
    </SideDrawer>
  </div>


</template>

