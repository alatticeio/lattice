<script setup>
import { computed } from 'vue'
import Icon from '../icons/Icon.vue'

const props = defineProps({
  node: { type: Object, default: null },
})
const emit = defineEmits(['close'])

const statusBadge = computed(() => {
  if (!props.node) return 'badge-ghost'
  return props.node.status === 'online' ? 'badge-success' : 'badge-ghost'
})
</script>

<template>
  <div class="p-4">
    <div class="flex items-start justify-between gap-3">
      <div>
        <div class="text-sm opacity-70">节点详情</div>
        <div class="text-lg font-semibold mt-1">
          {{ props.node?.name || '未选择节点' }}
        </div>
      </div>

      <button class="btn btn-ghost btn-sm" @click="emit('close')">关闭</button>
    </div>

    <div v-if="!props.node" class="mt-4 alert">
      <span>点击拓扑中的节点以查看详情。</span>
    </div>

    <div v-else class="mt-4 space-y-4">
      <div class="rounded-box border border-base-300 bg-base-100 p-4">
        <div class="flex items-center justify-between">
          <div class="font-medium">基本信息</div>
          <span class="badge" :class="statusBadge">{{ props.node.status }}</span>
        </div>

        <div class="mt-3 grid grid-cols-1 gap-2 text-sm">
          <div class="flex justify-between">
            <span class="opacity-70">IP</span>
            <span class="font-mono">{{ props.node.ip }}</span>
          </div>
          <div class="flex justify-between">
            <span class="opacity-70">角色</span>
            <span class="badge badge-outline">{{ props.node.role }}</span>
          </div>
          <div class="flex justify-between">
            <span class="opacity-70">Node ID</span>
            <span class="font-mono">{{ props.node.id }}</span>
          </div>
        </div>
      </div>

      <div class="rounded-box border border-base-300 bg-base-100 p-4">
        <div class="font-medium">快捷操作</div>
        <div class="mt-3 grid grid-cols-1 gap-2">
          <button class="btn btn-sm btn-primary">
            <Icon name="monitor" class="w-4 h-4" />
            查看监控
          </button>
          <button class="btn btn-sm btn-outline">
            <Icon name="policy" class="w-4 h-4" />
            查看策略命中
          </button>
          <button class="btn btn-sm btn-ghost text-error">
            <Icon name="logout" class="w-4 h-4" />
            下线节点（示意）
          </button>
        </div>
      </div>

      <div class="rounded-box bg-base-200 p-4 text-sm">
        <div class="font-medium">说明</div>
        <div class="opacity-70 mt-1">
          后续可在这里接入：Peer 列表、路由表、最近握手、事件流、链路质量历史等。
        </div>
      </div>
    </div>
  </div>
</template>