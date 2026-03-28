<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { add, listUser, deleteUser, type Member } from '@/api/user'
import { listWs } from '@/api/workspace'
import SideDrawer from '@/components/SideDrawer.vue'
import Pagination from '@/components/Pagination.vue'
import Icon from "@/components/icons/Icon.vue"
import { useAction, useTable } from '@/composables/useApi'
import { getFirstChar, getAvatarColor } from '@/composables/useTheme'
import { useConfirm } from '@/composables/useConfirm'

const { confirm } = useConfirm()

// --- API 数据流 ---
const { rows: members, total, loading, params, refresh } = useTable(listUser, {
  successMsg: '成员列表已更新',
})

const { rows: workspaces, refresh: wsRefresh } = useTable(listWs)

const { loading: addLoading, execute: runAdd } = useAction(add, {
  onSuccess: () => {
    isDrawerOpen.value = false
    refresh()
  }
})

// --- 状态控制 ---
const isDrawerOpen = ref(false)
const drawerType = ref<'invite' | 'config'>('invite')
const selectedMember = ref<Member | null>(null)
const isAddingNs = ref(false)
const newNsSelection = ref('')

const form = ref({
  email: '', username: '', password: '', role: 'viewer', namespace: '', provider: 'local' as 'local' | 'dex'
})

// --- 样式映射 (适配深色模式) ---
const roleTemplates = {
  admin: 'text-primary bg-primary/10 border-primary/20',
  editor: 'text-success bg-success/10 border-success/20',
  viewer: 'text-base-content/60 bg-base-content/5 border-base-content/10'
}

// 动态计算 Badge 颜色 (改用更柔和的半透明背景)
const getBadgeStyles = (name: string) => {
  const hash = name.split('').reduce((acc, char) => char.charCodeAt(0) + ((acc << 5) - acc), 0)
  const hues = [210, 160, 260, 40, 0, 190, 230] // 蓝, 绿, 紫, 黄, 红, 青, 靛
  const hue = hues[Math.abs(hash) % hues.length]
  return {
    backgroundColor: `hsla(${hue}, 70%, 50%, 0.15)`,
    color: `hsla(${hue}, 80%, 60%, 1)`,
    borderColor: `hsla(${hue}, 70%, 50%, 0.2)`
  }
}

// --- 逻辑处理 ---
const openConfig = (m: Member) => {
  selectedMember.value = JSON.parse(JSON.stringify(m))
  drawerType.value = 'config'
  isDrawerOpen.value = true
}

const openCreate = () => {
  drawerType.value = 'invite'
  isDrawerOpen.value = true
}

const handleDelete = async (user: Member) => {
  const isConfirmed = await confirm({
    title: '确认移除成员？',
    message: `确认从团队中移除 <span class="text-error font-bold">${user.name}</span>？此操作将同步撤销其在 K8s 集群中的 RBAC 绑定。`,
    confirmText: '确认移除',
    type: 'danger'
  })

  if (isConfirmed) {
    await deleteUser(user.id)
    refresh()
  }
}

const handleSave = async () => {
  await runAdd(form.value)
}
</script>

<template>
  <div class="max-w-7xl mx-auto p-4 lg:p-8 space-y-6 text-base-content">

    <header class="flex items-center justify-between pb-6 border-b border-base-content/10">
      <div class="flex items-center gap-4">
        <div class="p-3 bg-primary rounded-2xl text-primary-content shadow-lg shadow-primary/20">
          <Icon name="user" class="w-6 h-6"/>
        </div>
        <div>
          <h1 class="text-2xl font-black tracking-tighter uppercase">用户管理</h1>
          <p class="text-xs font-bold opacity-40 uppercase tracking-widest">Wireflow RBAC Control Plane</p>
        </div>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-ghost bg-base-200/50 border-base-content/5 rounded-xl hover:bg-base-200" @click="refresh">
          <Icon name="refresh" :class="['w-4 h-4']" />
        </button>
        <button @click="openCreate" class="btn btn-primary rounded-xl px-6 shadow-lg shadow-primary/20">
          <Icon name="plus" class="w-4 h-4 mr-2"/>
          添加成员
        </button>
      </div>
    </header>

    <div class="bg-base-100 border border-base-content/10 rounded-2xl shadow-sm overflow-hidden transition-all">
      <div class="overflow-x-auto">
        <table class="table table-zebra w-full">
          <thead class="bg-base-200/50 border-b border-base-content/5">
          <tr class="text-[10px] font-black uppercase tracking-widest opacity-50">
            <th class="px-6 py-4">Identity</th>
            <th class="px-6 py-4">Authorized Workspaces</th>
            <th class="px-6 py-4">System Role</th>
            <th class="px-6 py-4 text-right">Actions</th>
          </tr>
          </thead>

          <tbody class="divide-y divide-base-content/5">
          <tr v-if="members.length === 0">
            <td colspan="4" class="py-24 text-center">
              <div class="flex flex-col items-center opacity-20">
                <Icon name="user" class="w-16 h-16 mb-4"/>
                <p class="text-sm font-black uppercase tracking-widest">No Members Found</p>
              </div>
            </td>
          </tr>

          <tr v-for="m in members" :key="m.id" class="hover:bg-base-200/30 transition-colors">
            <td class="px-6 py-4">
              <div class="flex items-center gap-3">
                <div :class="['w-8 h-8 flex items-center justify-center rounded-lg text-[10px] font-black text-white shadow-inner', getAvatarColor(m.name)]">
                  {{ getFirstChar(m.name) }}
                </div>
                <div class="flex flex-col">
                  <span class="text-sm font-bold tracking-tight">{{ m.name }}</span>
                  <span class="text-[10px] opacity-40 font-mono">{{ m.email }}</span>
                </div>
              </div>
            </td>

            <td class="px-6 py-4">
              <div class="flex flex-wrap gap-1.5">
                  <span v-for="ws in m.workspaces" :key="ws.id"
                        :style="getBadgeStyles(ws.displayName)"
                        class="px-2 py-0.5 rounded-md text-[10px] font-bold border transition-colors">
                    {{ ws.displayName }}
                  </span>
                <span v-if="!m.workspaces?.length" class="text-[10px] italic opacity-30">Unassigned</span>
              </div>
            </td>

            <td class="px-6 py-4">
                <span :class="['px-2.5 py-1 text-[10px] font-black rounded-lg border uppercase tracking-tighter', roleTemplates[m.role as keyof typeof roleTemplates]]">
                  {{ m.role }}
                </span>
            </td>

            <td class="px-6 py-4">
              <div class="flex items-center justify-end gap-2">
                <button @click="openConfig(m)" class="btn btn-ghost btn-xs text-primary font-bold hover:bg-primary/10">
                  <Icon name="settings" class="w-3.5 h-3.5 mr-1" />
                  授权
                </button>
                <button @click="handleDelete(m)" class="btn btn-ghost btn-xs text-error/40 hover:text-error hover:bg-error/10">
                  <Icon name="trash" class="w-4 h-4" />
                </button>
              </div>
            </td>
          </tr>
          </tbody>
        </table>
      </div>

      <div class="p-4 bg-base-200/30 border-t border-base-content/5">
        <Pagination v-model:page="params.page" v-model:pageSize="params.pageSize" :total="total" item-name="成员" />
      </div>
    </div>

    <SideDrawer v-model="isDrawerOpen" :title="drawerType === 'invite' ? 'Provision Identity' : 'RBAC Mapping'">
      <div class="flex flex-col h-full bg-base-100">

        <div v-if="drawerType === 'invite'" class="flex-1 p-8 space-y-8 overflow-y-auto">
          <div class="bg-base-200/50 p-1.5 rounded-2xl flex gap-1 border border-base-content/5">
            <button @click="form.provider = 'local'" :class="form.provider === 'local' ? 'bg-base-100 text-primary shadow-sm' : 'opacity-50'" class="flex-1 py-2.5 rounded-xl text-xs font-black transition-all uppercase tracking-widest">Local</button>
            <button @click="form.provider = 'dex'" :class="form.provider === 'dex' ? 'bg-base-100 text-primary shadow-sm' : 'opacity-50'" class="flex-1 py-2.5 rounded-xl text-xs font-black transition-all uppercase tracking-widest">OIDC / Dex</button>
          </div>

          <div class="space-y-6">
            <div class="space-y-2">
              <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">Email Address</label>
              <input v-model="form.username" type="text" placeholder="ops@wireflow.io" class="input input-bordered w-full bg-base-200 border-none font-bold" />
            </div>

            <div v-if="form.provider === 'local'" class="space-y-2">
              <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">Initial Password</label>
              <input v-model="form.password" type="password" class="input input-bordered w-full bg-base-200 border-none font-mono" />
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div class="space-y-2">
                <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">System Role</label>
                <select v-model="form.role" class="select select-bordered w-full bg-base-200 border-none font-bold">
                  <option v-for="(_, r) in roleTemplates" :key="r" :value="r">{{ r.toUpperCase() }}</option>
                </select>
              </div>
              <div class="space-y-2">
                <label class="text-[10px] font-black opacity-40 uppercase tracking-widest px-1">Primary Workspace</label>
                <select v-model="form.namespace" class="select select-bordered w-full bg-base-200 border-none font-bold">
                  <option v-for="w in workspaces" :key="w.id" :value="w.id">{{ w.displayName }}</option>
                </select>
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="selectedMember" class="flex-1 p-8 space-y-8 overflow-y-auto">
          <div class="flex items-center gap-4 p-6 bg-primary rounded-3xl text-primary-content shadow-xl shadow-primary/20">
            <div :class="['w-12 h-12 flex items-center justify-center rounded-xl text-lg font-black bg-white/20 shadow-inner']">
              {{ getFirstChar(selectedMember.name) }}
            </div>
            <div>
              <div class="font-black italic text-lg uppercase tracking-tighter">{{ selectedMember.name }}</div>
              <div class="text-[10px] opacity-70 font-mono tracking-widest">{{ selectedMember.email }}</div>
            </div>
          </div>

          <section class="space-y-4">
            <div class="flex justify-between items-center">
              <h3 class="text-[10px] font-black opacity-40 uppercase tracking-[0.2em]">Workspace Bindings</h3>
              <button @click="isAddingNs = !isAddingNs" class="btn btn-ghost btn-xs text-primary font-black">+ BIND</button>
            </div>

            <div v-if="isAddingNs" class="p-4 bg-primary/5 rounded-2xl border border-primary/20 space-y-4 animate-in zoom-in-95">
              <select v-model="newNsSelection" class="select select-sm w-full bg-base-100 font-bold">
                <option value="">Select Target...</option>
                <option v-for="w in workspaces" :key="w.id" :value="w.id">{{ w.displayName }}</option>
              </select>
              <div class="flex gap-2">
                <button class="btn btn-primary btn-xs flex-1 rounded-lg">Confirm</button>
                <button @click="isAddingNs = false" class="btn btn-ghost btn-xs">Cancel</button>
              </div>
            </div>

            <div class="grid gap-3">
              <div v-for="i in 2" :key="i" class="p-4 rounded-2xl border border-base-content/10 bg-base-200/30 hover:border-primary/50 transition-all group">
                <div class="flex justify-between items-center mb-4">
                  <span class="text-xs font-black italic uppercase">Dev-Namespace-{{ i }}</span>
                  <button class="opacity-0 group-hover:opacity-100 btn btn-ghost btn-xs text-error"><Icon name="trash" class="w-3.5 h-3.5"/></button>
                </div>
                <div class="grid grid-cols-2 gap-4">
                  <div class="space-y-1">
                    <label class="text-[9px] font-black opacity-30 uppercase">RBAC Role</label>
                    <select class="select select-xs w-full bg-base-100 border-none font-bold">
                      <option>Namespace Admin</option>
                      <option>Editor</option>
                      <option>Viewer</option>
                    </select>
                  </div>
                  <div class="space-y-1">
                    <label class="text-[9px] font-black opacity-30 uppercase">Quota Set</label>
                    <div class="text-[10px] font-mono font-bold mt-1.5 opacity-60 italic tracking-tighter">Standard-Burst</div>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div class="p-8 bg-base-200/50 border-t border-base-content/5 flex gap-3">
          <button @click="isDrawerOpen = false" class="btn btn-ghost flex-1 rounded-2xl font-black uppercase tracking-widest text-xs">Cancel</button>
          <button @click="handleSave" class="btn btn-primary flex-[2] rounded-2xl font-black uppercase tracking-widest text-xs shadow-xl shadow-primary/20">
            {{ drawerType === 'invite' ? 'Provision Identity' : 'Sync RBAC' }}
          </button>
        </div>
      </div>
    </SideDrawer>
  </div>
</template>

<style scoped>
.table :where(thead, tfoot) :where(th, td) {
  background-color: transparent;
}
</style>