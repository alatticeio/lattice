<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Shield } from 'lucide-vue-next'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { useDrawer } from '@/composables/useAgentDetailDrawer'

const { t } = useI18n()
const drawer = useDrawer()

// PRO 检测：优先环境变量，回退懒检查
const isPro = ref(import.meta.env.VITE_EDITION === 'pro')

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString()
}

function formatBytes(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} KB`
  return `${bytes} B`
}
</script>

<template>
  <div>
    <!-- PRO 未启用 -->
    <div v-if="!isPro" class="flex flex-col items-center justify-center py-12 gap-3">
      <Shield class="w-10 h-10 text-muted-foreground" />
      <p class="text-sm text-muted-foreground">{{ t('agent.networkPro') }}</p>
      <Button variant="outline" size="sm" as="a" href="/billing">Upgrade</Button>
    </div>

    <!-- 未选中 trace -->
    <div v-else-if="!drawer.selectedTraceId.value" class="flex items-center justify-center py-12">
      <p class="text-sm text-muted-foreground">{{ t('agent.networkNoTrace') }}</p>
    </div>

    <!-- 加载中 -->
    <div v-else-if="drawer.flowLoading.value" class="space-y-2">
      <Skeleton v-for="i in 3" :key="i" class="h-8 w-full" />
    </div>

    <!-- 错误 -->
    <div v-else-if="drawer.flowError.value" class="flex flex-col items-center py-8 gap-2">
      <p class="text-sm text-red-500">{{ drawer.flowError.value }}</p>
      <Button variant="ghost" size="sm" @click="drawer.loadFlowEvents()">Retry</Button>
    </div>

    <!-- 空数据 -->
    <div v-else-if="drawer.flowEvents.value.length === 0" class="flex items-center justify-center py-12">
      <p class="text-sm text-muted-foreground">{{ t('agent.networkEmpty') }}</p>
    </div>

    <!-- 表格 -->
    <Table v-else>
      <TableHeader>
        <TableRow>
          <TableHead>Time</TableHead>
          <TableHead>Direction</TableHead>
          <TableHead>Destination</TableHead>
          <TableHead class="text-right">Bytes</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="event in drawer.flowEvents.value" :key="`${event.ts}-${event.dstPort}`">
          <TableCell class="text-xs">{{ formatTime(event.ts) }}</TableCell>
          <TableCell>
            <Badge :variant="event.direction === 'egress' ? 'outline' : 'secondary'">
              {{ event.direction }}
            </Badge>
          </TableCell>
          <TableCell class="text-xs font-mono">{{ event.dstIp }}:{{ event.dstPort }}</TableCell>
          <TableCell class="text-xs text-right tabular-nums">{{ formatBytes(event.bytes) }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
