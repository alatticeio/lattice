<script setup lang="ts">
import {inject, ref} from 'vue';
import {create, rmToken, listTokens} from '@/api/token';
import {useConfirm} from '@/composables/useConfirm' // 引入插件
import Pagination from '@/components/Pagination.vue'
import {useAction, useTable} from '@/composables/useApi'
import {getFirstChar, getAvatarColor} from '@/composables/useTheme'

const {confirm} = useConfirm()
const showModal = ref(false);
// 注入全局 Toast 函数
const toast = inject('globalToast')
const installCommand = ref("curl -sL https://wireflow.run/install.sh | sh -s -- --token YOUR_TOKEN_HERE");
const copied = ref(false);

// 1. 列表自动管理（含自动报错 Toast）
const {rows, total, loading, params, refresh} = useTable(listTokens, {
  successMsg: '刷新列表成功',
  errorMsg: '刷新列表失败',
  initialParams: {namespace: 'wf-test'}
})

const {loading: addLoading, execute: runCreate} = useAction(create, {
  successMsg: "生成Token成功",
  errorMsg: "生成Token失败",
})

const form = ref({
  name: '',
})

const {loading: deleteLoading, execute: runDelete} = useAction(rmToken, {
  successMsg: "删除Token成功",
  errorMsg: "删除Token失败",
})

const handleCreateToken = async () => {
  // 像写同步代码一样调用弹窗
  const isConfirmed = await confirm({
    title: '确认生成Token？',
    message: `你正在尝试创建一个Token，请确认此操作`,
    confirmText: '立即生成',
    type: 'danger'
  })
  if (isConfirmed) {
    await runCreate()
    refresh()
  }
}

const handleCopyCommand = async (token) => {
  // 这里可以根据选中的 token 动态生成命令
  installCommand.value = `curl -sL https://wireflow.io/i.sh | sh -s -- --token ${token}`;
  showModal.value = true;
  try {
    await navigator.clipboard.writeText(installCommand.value);
    copied.value = true;
    setTimeout(() => (copied.value = false), 3000);
  } catch (err) {
    console.error('复制失败', err);
  }
};


const handleDelete = async (token) => {
  // 像写同步代码一样调用弹窗
  const isConfirmed = await confirm({
    title: '确认删除Token？',
    message: `你正在尝试删除Token <span class="text-error font-bold">${token}</span>。此操作不可撤销。`,
    confirmText: '立即销毁',
    type: 'danger'
  })

  if (isConfirmed) {
    loading.value = true
    try {
      // 调用你的删除 API
      await runDelete(token)
      toast("Token deleted successfully")
      await refresh() // 刷新列表
    } finally {
      loading.value = false
    }
  }
}

</script>

<template>
  <div class="max-w-7xl mx-auto p-4 lg:p-8 space-y-6">

    <div
        class="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 border-b border-base-300 pb-6">
      <div>
        <h1 class="text-3xl font-extrabold tracking-tight flex items-center gap-3">
          <div class="p-2 bg-warning/10 rounded-xl">
            <svg class="w-8 h-8 text-warning" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/>
            </svg>
          </div>
          Token 身份管理
        </h1>
        <p class="text-sm opacity-50 mt-2 max-w-xl leading-relaxed">
          注册令牌 (Enrollment Tokens) 用于节点接入集群。Token 具有时效性，请确保在受信任的环境中使用并妥善保管。
        </p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-ghost border-base-300 rounded-xl" @click="refresh">
          <svg :class="['w-4 h-4', loading ? 'animate-spin' : '']" fill="none" stroke="currentColor"
               viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
          </svg>
        </button>
        <button class="btn btn-primary btn-md shadow-lg shadow-primary/20 rounded-xl px-8" @click="handleCreateToken">
          创建 Token
        </button>
      </div>
    </div>

    <div class="alert bg-warning/5 border-warning/20 text-warning-content rounded-2xl flex items-start gap-4">
      <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-6 w-6" fill="none" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
      </svg>
      <div class="text-xs">
        <h4 class="font-bold">安全建议</h4>
        <p class="opacity-70">Token 仅在生成时完整显示，之后将进行脱敏处理。请勿在非公开场合分享您的安装命令。</p>
      </div>
    </div>

    <div class="bg-base-100 rounded-2xl border border-base-300 shadow-sm overflow-hidden">
      <div class="p-4 bg-base-200/30 border-b border-base-300 flex items-center gap-4">
        <div class="relative flex-1 w-full md:max-w-md">
          <svg class="w-4 h-4 absolute left-3 top-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
          </svg>
          <input v-model="params.search"
                 @keyup.enter="refresh"
                 class="input input-bordered input-sm w-full pl-10 bg-base-100 focus:border-primary"
                 placeholder="搜索Token名称..."/>
          <button
              @click="refresh"
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
        <div v-if="loading" class="p-20 text-center"><span
            class="loading loading-spinner loading-lg text-primary"></span></div>

        <div v-for="r in rows" :key="r.token"
             class="group flex flex-col md:flex-row items-center justify-between gap-4 p-4 px-6 hover:bg-base-200/50 transition-colors">

          <div class="flex items-center gap-4 w-full md:w-2/5">
            <div class="avatar placeholder">
              <div
                  :class="[
    'w-6 h-6 flex items-center justify-center rounded-md text-[10px] font-bold text-white shadow-sm transition-transform group-hover:scale-110',
    getAvatarColor(r.token)
  ]"
              >
                {{ getFirstChar(r.token) }}
              </div>
            </div>
            <div class="truncate">
              <div class="font-mono text-sm font-bold flex items-center gap-2">
                {{ r.token }}
                <div v-if="r.status === 'active'" class="w-1.5 h-1.5 rounded-full bg-success animate-pulse"></div>
              </div>
              <div class="flex gap-2 mt-1">
                <span class="badge badge-ghost badge-xs opacity-60">Namespace: {{ r.namespace }}</span>
              </div>
            </div>
          </div>

          <div class="hidden md:flex flex-1 flex-col text-xs">
            <span class="opacity-40 uppercase font-bold text-[9px]">Expires at</span>
            <span class="font-mono opacity-70">{{ r.expiry }}</span>
          </div>

          <div class="flex items-center gap-2 shrink-0 w-full md:w-auto justify-end">
            <button class="btn btn-primary btn-sm rounded-lg flex gap-2" @click="handleCopyCommand(r.token)">
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24"
                   stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
              </svg>
              一键加入
            </button>
            <div class="divider divider-horizontal mx-1"></div>
            <button class="btn btn-ghost btn-sm text-error/40 hover:text-error hover:bg-error/10"
                    @click="handleDelete(r.token)">删除
            </button>
          </div>
        </div>

        <!-- 分页部分 -->
        <Pagination
            v-model:page="params.page"
            v-model:pageSize="params.pageSize"
            :total="total"
            item-name="密钥"
        />

        <div v-if="rows.length === 0" class="p-20 text-center opacity-30 flex flex-col items-center">
          <svg class="w-12 h-12 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
                d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
          </svg>
          <p class="text-sm font-bold">未找到匹配的密钥</p>
        </div>

      </div>
    </div>

    <Teleport to="body">
      <dialog :class="['modal', showModal ? 'modal-open' : '']">
        <div class="modal-box w-11/12 max-w-3xl p-0 overflow-hidden rounded-2xl border border-base-300">
          <div class="p-6 bg-base-200/50 border-b border-base-300">
            <h3 class="font-bold text-xl flex items-center gap-2">
              <svg class="w-6 h-6 text-success" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
              </svg>
              Token 已准备就绪
            </h3>
            <p class="text-xs opacity-50 mt-1">请在您的目标 Linux 服务器终端执行以下命令进行节点绑定。</p>
          </div>

          <div class="p-8 space-y-4">
            <div class="relative">
              <div class="mockup-code bg-[#1e1e1e] text-emerald-400 p-6 shadow-2xl">
                <pre data-prefix="$"><code>{{ installCommand }}</code></pre>
              </div>
              <button
                  @click="handleCopyCommand"
                  :class="['btn btn-sm absolute top-4 right-4', copied ? 'btn-success' : 'btn-primary']"
              >
                {{ copied ? '已复制!' : '复制命令' }}
              </button>
            </div>

            <div class="flex items-center gap-2 text-[10px] opacity-40 justify-center">
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
              </svg>
              支持架构: x86_64, ARM64 | 运行环境: Linux (Systemd)
            </div>
          </div>

          <div class="modal-action p-4 bg-base-200/50 border-t border-base-300">
            <button class="btn btn-ghost px-8" @click="showModal = false">完成并关闭</button>
          </div>
        </div>
      </dialog>
    </Teleport>

    <div v-if="copied" class="toast toast-top toast-center z-50">
      <div class="alert alert-success shadow-lg border-none rounded-xl">
        <span class="text-xs font-bold">命令已同步至剪贴板</span>
      </div>
    </div>
  </div>
</template>