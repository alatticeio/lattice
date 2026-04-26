<script setup lang="ts">
import { ref } from 'vue'
import { ChevronRight, Wrench } from 'lucide-vue-next'
import type { ToolCall } from '@/stores/useAiStore'

const props = defineProps<{
  toolCall: ToolCall
  streaming?: boolean
}>()

const open = ref(false)

const toolLabels: Record<string, string> = {
  list_peers:         '查询 Peer 列表',
  list_policies:      '查询策略列表',
  list_networks:      '查询网络列表',
  check_connectivity: '检查连通性',
}

const label = props.toolCall.tool in toolLabels
  ? toolLabels[props.toolCall.tool]
  : props.toolCall.tool
</script>

<template>
  <div class="inline-flex flex-col">
    <button
      class="group inline-flex items-center gap-1.5 rounded-full border border-border bg-muted/60 px-3 py-1 text-xs text-muted-foreground transition-colors hover:border-primary/30 hover:bg-muted hover:text-foreground"
      @click="open = !open"
    >
      <Wrench class="size-3 shrink-0" />
      <span>{{ label }}</span>
      <!-- Streaming pulse -->
      <span v-if="streaming" class="size-1.5 rounded-full bg-primary animate-pulse" />
      <!-- Toggle arrow -->
      <ChevronRight
        v-else
        class="size-3 shrink-0 transition-transform"
        :class="{ 'rotate-90': open }"
      />
    </button>

    <!-- Expanded detail -->
    <div
      v-if="open"
      class="mt-1.5 rounded-lg border border-border bg-muted/40 px-3 py-2.5"
    >
      <pre class="text-[11px] text-muted-foreground leading-relaxed whitespace-pre-wrap break-all font-mono">{{ JSON.stringify(toolCall.input, null, 2) }}</pre>
    </div>
  </div>
</template>
