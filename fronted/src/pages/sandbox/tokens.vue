<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Key, Loader2, Copy, Check, X, Plus } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { useSandboxStore } from '@/stores/useSandboxStore'
import type { EnrollmentToken } from '@/api/sandbox'
import { listTools } from '@/api/tools'
import type { MCPTool } from '@/api/tools'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'common.sandbox.tokens.title', descKey: 'common.sandbox.tokens.desc' },
})

const { t } = useI18n()
const store = useSandboxStore()

const ttlOptions = [
  { value: 3600, label: 'common.sandbox.tokens.ttl1h' },
  { value: 21600, label: 'common.sandbox.tokens.ttl6h' },
  { value: 86400, label: 'common.sandbox.tokens.ttl24h' },
]

const toolOptions = ref<MCPTool[]>([])
const toolsLoading = ref(false)

async function fetchTools() {
  toolsLoading.value = true
  try {
    toolOptions.value = await listTools()
  } catch {
    // non-critical, leave empty
  } finally {
    toolsLoading.value = false
  }
}

const ttl = ref(3600)
const allowedTools = ref<string[]>(toolOptions.value.slice(0, 2).map(t => t.name))
const creating = ref(false)

const generatedToken = ref<EnrollmentToken | null>(null)
const tokenDialog = ref(false)
const copied = ref(false)

function toggleTool(tool: string) {
  const idx = allowedTools.value.indexOf(tool)
  if (idx >= 0) allowedTools.value.splice(idx, 1)
  else allowedTools.value.push(tool)
}

async function handleCreate() {
  creating.value = true
  try {
    const result = await store.generateToken({
      allowedTools: allowedTools.value,
      ttlSeconds: ttl.value,
    })
    generatedToken.value = result
    tokenDialog.value = true
  } catch (e: any) {
    toast.error(e?.message || t('common.sandbox.tokens.error'))
  } finally {
    creating.value = false
  }
}

async function copyToken() {
  if (!generatedToken.value?.token) return
  await navigator.clipboard.writeText(generatedToken.value.token)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

function closeTokenDialog() {
  tokenDialog.value = false
  generatedToken.value = null
  copied.value = false
}

async function handleRevoke(token: string) {
  try {
    await store.revokeToken(token)
    toast.success(t('common.sandbox.tokens.revoke'))
  } catch (e: any) {
    toast.error(e?.message || t('common.sandbox.tokens.error'))
  }
}

const statusClass = (status: string) => {
  const map: Record<string, string> = {
    active: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    expired: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    revoked: 'bg-muted text-muted-foreground',
  }
  return map[status] || map.revoked
}

onMounted(() => { store.fetchTokens(); fetchTools() })
</script>

<template>
  <div class="p-6 space-y-6">
    <!-- Create section -->
    <div class="rounded-xl border border-border bg-card p-6">
      <h3 class="mb-1 font-semibold">{{ t('common.sandbox.tokens.createTitle') }}</h3>
      <p class="text-muted-foreground mb-4 text-sm">{{ t('common.sandbox.tokens.createDesc') }}</p>

      <div class="space-y-4">
        <!-- TTL -->
        <div>
          <label class="text-sm font-medium">{{ t('common.sandbox.tokens.ttl') }}</label>
          <div class="flex gap-2 mt-1.5">
            <Button
              v-for="opt in ttlOptions" :key="opt.value"
              :variant="ttl === opt.value ? 'default' : 'outline'"
              size="sm"
              @click="ttl = opt.value"
            >
              {{ t(opt.label) }}
            </Button>
          </div>
        </div>

        <!-- Allowed Tools -->
        <div>
          <label class="text-sm font-medium">{{ t('common.sandbox.tokens.allowedTools') }}</label>
          <div class="flex flex-wrap gap-2 mt-1.5">
            <template v-if="toolsLoading">
              <span class="text-xs text-muted-foreground">Loading tools...</span>
            </template>
            <template v-else-if="toolOptions.length === 0">
              <span class="text-xs text-muted-foreground">No tools available</span>
            </template>
            <button
              v-for="tool in toolOptions" :key="tool.name"
              class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs transition-colors"
              :class="allowedTools.includes(tool.name)
                ? 'bg-primary/10 border-primary/30 text-primary'
                : 'border-border text-muted-foreground hover:bg-muted'"
              @click="toggleTool(tool.name)"
            >
              {{ tool.name }}
              <span v-if="tool.description" class="text-[10px] text-muted-foreground/60 truncate max-w-24">{{ tool.description }}</span>
              <Check v-if="allowedTools.includes(tool.name)" class="size-3" />
            </button>
          </div>
        </div>

        <Button :disabled="creating || allowedTools.length === 0" @click="handleCreate">
          <Loader2 v-if="creating" class="mr-2 size-4 animate-spin" />
          <Plus v-else class="mr-2 size-4" />
          {{ t('common.sandbox.tokens.generate') }}
        </Button>
      </div>
    </div>

    <!-- Token list -->
    <div v-if="store.tokensLoading" class="flex gap-4">
      <div v-for="i in 3" :key="i" class="h-12 flex-1 animate-pulse rounded-lg bg-muted" />
    </div>

    <div v-else-if="store.tokens.length === 0" class="flex flex-col items-center gap-2 py-12 text-sm text-muted-foreground">
      <Key class="size-10 opacity-40" />
      <p>{{ t('common.sandbox.tokens.empty') }}</p>
    </div>

    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('common.sandbox.tokens.colToken') }}</TableHead>
            <TableHead>{{ t('common.sandbox.tokens.colCreated') }}</TableHead>
            <TableHead>{{ t('common.sandbox.tokens.colExpires') }}</TableHead>
            <TableHead>{{ t('common.sandbox.tokens.colStatus') }}</TableHead>
            <TableHead>{{ t('common.sandbox.tokens.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="tok in store.tokens" :key="tok.maskedToken">
            <TableCell class="font-mono text-xs">{{ tok.maskedToken }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(tok.createdAt).toLocaleString() }}</TableCell>
            <TableCell class="text-xs text-muted-foreground">{{ new Date(tok.expiresAt).toLocaleString() }}</TableCell>
            <TableCell>
              <Badge :class="statusClass(tok.status)" variant="secondary">
                {{ t(`common.sandbox.tokens.status${tok.status.charAt(0).toUpperCase() + tok.status.slice(1)}`) }}
              </Badge>
            </TableCell>
            <TableCell>
              <Button
                v-if="tok.status === 'active'"
                variant="ghost" size="sm"
                class="text-destructive hover:text-destructive text-xs"
                @click="handleRevoke(tok.token || tok.maskedToken)"
              >
                <X class="mr-1 size-3" />
                {{ t('common.sandbox.tokens.revoke') }}
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Token dialog -->
    <Dialog :open="tokenDialog" @update:open="closeTokenDialog">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('common.sandbox.tokens.generatedToken') }}</DialogTitle>
          <DialogDescription class="text-amber-600 dark:text-amber-400">
            {{ t('common.sandbox.tokens.tokenWarning') }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex items-center gap-2 rounded-lg bg-muted p-3 font-mono text-sm break-all">
          <code class="flex-1 text-xs">{{ generatedToken?.token }}</code>
          <Button variant="ghost" size="icon" class="shrink-0 size-8" @click="copyToken">
            <Check v-if="copied" class="size-4 text-emerald-500" />
            <Copy v-else class="size-4" />
          </Button>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="closeTokenDialog">{{ t('common.action.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
