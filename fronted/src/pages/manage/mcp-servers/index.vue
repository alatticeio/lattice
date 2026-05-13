<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Server, Loader2, Plus, Trash2, ExternalLink, Globe, Lock, Users } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { useMcpServerStore } from '@/stores/useMcpServerStore'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'manage.mcpServers.title', descKey: 'manage.mcpServers.desc' },
})

const { t } = useI18n()
const store = useMcpServerStore()

// ── Register form ──────────────────────────────────────────────
const registerDialog = ref(false)
const formName = ref('')
const formDesc = ref('')
const formURL = ref('')
const formVisibility = ref<'workspace' | 'private' | 'public'>('workspace')
const registering = ref(false)

const visibilityOptions = [
  { value: 'workspace' as const, icon: Users,  label: 'manage.mcpServers.visWorkspace' },
  { value: 'private'   as const, icon: Lock,   label: 'manage.mcpServers.visPrivate' },
  { value: 'public'    as const, icon: Globe,  label: 'manage.mcpServers.visPublic' },
]

function resetForm() {
  formName.value = ''
  formDesc.value = ''
  formURL.value = ''
  formVisibility.value = 'workspace'
}

async function handleRegister() {
  if (!formName.value.trim() || !formURL.value.trim()) return
  registering.value = true
  try {
    await store.register({
      name: formName.value.trim(),
      description: formDesc.value.trim(),
      mcpServerURL: formURL.value.trim(),
      visibility: formVisibility.value,
    })
    toast.success(t('manage.mcpServers.registerSuccess'))
    registerDialog.value = false
    resetForm()
  } catch (e: any) {
    toast.error(e?.message || t('manage.mcpServers.registerError'))
  } finally {
    registering.value = false
  }
}

async function handleDelete(name: string) {
  if (!confirm(t('manage.mcpServers.confirmDelete'))) return
  try {
    await store.remove(name)
    toast.success(t('manage.mcpServers.deleteSuccess'))
  } catch (e: any) {
    toast.error(e?.message || t('manage.mcpServers.deleteError'))
  }
}

const visibilityBadge = (v: string) => {
  const map: Record<string, string> = {
    workspace: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
    private: 'bg-muted text-muted-foreground',
    public: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  }
  return map[v] || map.workspace
}

onMounted(() => store.fetch())
</script>

<template>
  <div class="p-6 space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <p class="text-muted-foreground text-sm">{{ t('manage.mcpServers.desc') }}</p>
      </div>
      <Button @click="registerDialog = true">
        <Plus class="mr-2 size-4" />
        {{ t('manage.mcpServers.register') }}
      </Button>
    </div>

    <!-- Loading -->
    <div v-if="store.loading" class="flex gap-4">
      <div v-for="i in 3" :key="i" class="h-16 flex-1 animate-pulse rounded-xl bg-muted" />
    </div>

    <!-- Empty -->
    <div v-else-if="store.servers.length === 0" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <Server class="size-10 opacity-40" />
      <p>{{ t('manage.mcpServers.empty') }}</p>
      <Button variant="outline" size="sm" class="mt-2" @click="registerDialog = true">
        <Plus class="mr-1.5 size-3.5" />{{ t('manage.mcpServers.registerFirst') }}
      </Button>
    </div>

    <!-- Server list -->
    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('manage.mcpServers.colName') }}</TableHead>
            <TableHead>{{ t('manage.mcpServers.colDesc') }}</TableHead>
            <TableHead>{{ t('manage.mcpServers.colURL') }}</TableHead>
            <TableHead>{{ t('manage.mcpServers.colVisibility') }}</TableHead>
            <TableHead>{{ t('manage.mcpServers.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="srv in store.servers" :key="srv.name">
            <TableCell class="font-medium">{{ srv.name }}</TableCell>
            <TableCell class="text-sm text-muted-foreground max-w-[200px] truncate">{{ srv.description || '—' }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">
              <a :href="srv.mcpServerURL" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 hover:text-foreground transition-colors">
                {{ srv.mcpServerURL }}
                <ExternalLink class="size-3" />
              </a>
            </TableCell>
            <TableCell>
              <Badge :class="visibilityBadge(srv.visibility)" variant="secondary">
                {{ t(`manage.mcpServers.vis${srv.visibility.charAt(0).toUpperCase() + srv.visibility.slice(1)}`) }}
              </Badge>
            </TableCell>
            <TableCell>
              <Button variant="ghost" size="icon" class="size-8 text-muted-foreground hover:text-destructive" @click="handleDelete(srv.name)">
                <Trash2 class="size-4" />
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Register dialog -->
    <Dialog :open="registerDialog" @update:open="registerDialog = $event; if (!$event) resetForm()">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('manage.mcpServers.registerTitle') }}</DialogTitle>
          <DialogDescription>{{ t('manage.mcpServers.registerDesc') }}</DialogDescription>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-1.5">
            <Label>{{ t('manage.mcpServers.formName') }}</Label>
            <Input v-model="formName" :placeholder="t('manage.mcpServers.formNamePlaceholder')" />
          </div>
          <div class="space-y-1.5">
            <Label>{{ t('manage.mcpServers.formDesc') }}</Label>
            <Input v-model="formDesc" :placeholder="t('manage.mcpServers.formDescPlaceholder')" />
          </div>
          <div class="space-y-1.5">
            <Label>{{ t('manage.mcpServers.formURL') }}</Label>
            <Input v-model="formURL" placeholder="http://my-mcp-server:8080" />
          </div>
          <div class="space-y-1.5">
            <Label>{{ t('manage.mcpServers.formVisibility') }}</Label>
            <div class="flex gap-2">
              <Button
                v-for="opt in visibilityOptions" :key="opt.value"
                :variant="formVisibility === opt.value ? 'default' : 'outline'"
                size="sm"
                @click="formVisibility = opt.value"
              >
                <component :is="opt.icon" class="mr-1.5 size-3.5" />
                {{ t(opt.label) }}
              </Button>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="registerDialog = false">{{ t('common.action.cancel') }}</Button>
          <Button :disabled="registering || !formName.trim() || !formURL.trim()" @click="handleRegister">
            <Loader2 v-if="registering" class="mr-2 size-4 animate-spin" />
            <Plus v-else class="mr-2 size-4" />
            {{ t('manage.mcpServers.register') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
