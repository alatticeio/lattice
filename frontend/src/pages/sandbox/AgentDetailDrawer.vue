<script setup lang="ts">
import { provide } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Sheet, SheetContent, SheetHeader, SheetTitle,
} from '@/components/ui/sheet'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import TracesSplitPanel from './components/TracesSplitPanel.vue'
import NetworkFlowTable from './components/NetworkFlowTable.vue'
import SubAgentsPanel from './components/SubAgentsPanel.vue'
import { useAgentDetailDrawer, drawerKey } from '@/composables/useAgentDetailDrawer'

const { t } = useI18n()
const dr = useAgentDetailDrawer()
provide(drawerKey, dr)

const statusClass = (status: string) =>
  status === 'online'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-muted text-muted-foreground'

const tabs = [
  { key: 'traces' as const, label: 'agent.traces' },
  { key: 'network' as const, label: 'agent.network' },
  { key: 'subagents' as const, label: 'agent.subAgents' },
]
</script>

<template>
  <Sheet :open="dr.open.value" @update:open="(v: boolean) => !v && dr.closeDrawer()">
    <SheetContent side="right" class="flex w-[640px] flex-col gap-0 p-0 sm:max-w-[640px]">
      <!-- Header -->
      <SheetHeader class="border-b px-6 py-4">
        <SheetTitle class="flex items-center gap-2">
          <span class="font-mono text-sm">{{ dr.agent.value?.name }}</span>
          <Badge v-if="dr.agent.value" :class="statusClass(dr.agent.value.status)" variant="secondary">
            {{ dr.agent.value.status }}
          </Badge>
          <Badge v-if="dr.agent.value" variant="outline">
            {{ dr.agent.value.mode }}
          </Badge>
          <span class="text-xs text-muted-foreground font-mono">{{ dr.agent.value?.vpnIP }}</span>
        </SheetTitle>
      </SheetHeader>

      <!-- Stats 卡片 -->
      <div class="px-6 py-3 flex gap-4">
        <div v-if="dr.tracesLoading.value && dr.stats.value.total === 0" class="flex gap-4 flex-1">
          <Skeleton v-for="i in 3" :key="i" class="h-14 flex-1" />
        </div>
        <template v-else>
          <div class="flex-1 rounded-lg bg-muted/50 p-3 text-center">
            <div class="text-lg font-semibold tabular-nums">{{ dr.stats.value.total }}</div>
            <div class="text-xs text-muted-foreground">{{ t('agent.totalCalls') }}</div>
          </div>
          <div class="flex-1 rounded-lg bg-muted/50 p-3 text-center">
            <div class="text-lg font-semibold tabular-nums">{{ dr.stats.value.successRate }}</div>
            <div class="text-xs text-muted-foreground">{{ t('agent.successRate') }}</div>
          </div>
          <div class="flex-1 rounded-lg bg-muted/50 p-3 text-center">
            <div class="text-lg font-semibold tabular-nums">{{ dr.stats.value.blocked }}</div>
            <div class="text-xs text-muted-foreground">{{ t('agent.blocked') }}</div>
          </div>
        </template>
      </div>

      <!-- Tab 栏 -->
      <div class="flex gap-1 px-6 border-b">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="px-4 py-2 text-sm font-medium border-b-2 transition-colors"
          :class="dr.activeTab.value === tab.key
            ? 'border-primary text-foreground'
            : 'border-transparent text-muted-foreground hover:text-foreground'"
          @click="dr.switchTab(tab.key)"
        >
          {{ t(tab.label) }}
        </button>
      </div>

      <!-- Tab 内容 -->
      <div class="flex-1 overflow-y-auto px-6 py-4">
        <TracesSplitPanel v-if="dr.activeTab.value === 'traces'" />
        <NetworkFlowTable v-if="dr.activeTab.value === 'network'" />
        <SubAgentsPanel v-if="dr.activeTab.value === 'subagents'" />
      </div>
    </SheetContent>
  </Sheet>
</template>
