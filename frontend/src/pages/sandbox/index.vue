<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Container, Trash2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { useSandboxStore } from '@/stores/useSandboxStore'
import { useAgentDetailDrawer } from '@/composables/useAgentDetailDrawer'
import AgentDetailDrawer from './AgentDetailDrawer.vue'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'common.sandbox.list.title', descKey: 'common.sandbox.list.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()
const drawer = useAgentDetailDrawer()

onMounted(() => store.fetchSandboxes())

const statusClass = (status: string) =>
  status === 'online'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-muted text-muted-foreground'

const modeLabel = (mode: string) =>
  mode === 'gvisor' ? t('common.sandbox.list.modeGvisor') : t('common.sandbox.list.modeCgroup')

function formatBytes(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} KB`
  return `${bytes} B`
}

async function handleRevoke(name: string) {
  if (!confirm(`${t('common.sandbox.list.confirmRevokeDesc')}`)) return
  try {
    await store.revokeSandbox(name)
    toast.success(`${name} ${t('common.sandbox.list.revoke')}`)
  } catch (e: any) {
    toast.error(e?.message || t('common.sandbox.list.error'))
  }
}
</script>

<template>
  <div class="p-6 space-y-4">
    <div v-if="store.sandboxesLoading" class="flex gap-4">
      <div v-for="i in 3" :key="i" class="h-16 flex-1 animate-pulse rounded-xl bg-muted" />
    </div>

    <div v-else-if="store.sandboxes.length === 0" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <Container class="size-10 opacity-40" />
      <p>{{ t('common.sandbox.list.empty') }}</p>
    </div>

    <div v-else class="lattice-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('common.sandbox.list.colName') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colStatus') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colMode') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colIP') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colSandboxId') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colTrafficRx') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colTrafficTx') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colCreated') }}</TableHead>
            <TableHead>{{ t('common.sandbox.list.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
          v-for="s in store.sandboxes"
          :key="s.name"
          class="cursor-pointer hover:bg-muted/50"
          @click="drawer.openDrawer(s)"
        >
            <TableCell class="font-medium">{{ s.name }}</TableCell>
            <TableCell>
              <Badge :class="statusClass(s.status)" variant="secondary">
                {{ t(`common.sandbox.list.${s.status}`) }}
              </Badge>
            </TableCell>
            <TableCell class="text-sm">{{ modeLabel(s.mode) }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ s.vpnIP }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">{{ s.sandboxId }}</TableCell>
            <TableCell class="text-sm">{{ formatBytes(s.trafficRx) }}</TableCell>
            <TableCell class="text-sm">{{ formatBytes(s.trafficTx) }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(s.createdAt).toLocaleString() }}</TableCell>
            <TableCell>
              <Button variant="ghost" size="icon" class="size-8 text-muted-foreground hover:text-destructive" @click="handleRevoke(s.name)">
                <Trash2 class="size-4" />
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
    <AgentDetailDrawer />
  </div>
</template>
