<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import {
  ShieldCheck, Plus, Search, Loader2, Trash2, Eye, Pencil,
  ShieldAlert, ShieldOff, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { useAgentPolicyStore } from '@/stores/useAgentPolicyStore'

definePage({
  meta: { titleKey: 'manage.agentPolicies.title', descKey: 'manage.agentPolicies.desc' },
})

const { t } = useI18n()
const store = useAgentPolicyStore()

// ── Search / filter ────────────────────────────────────────────
const searchQuery = ref('')
const filteredRows = computed(() => {
  const q = searchQuery.value.toLowerCase()
  if (!q) return store.rows
  return store.rows.filter(p =>
    p.name.toLowerCase().includes(q) ||
    store.selectorToString(p.spec.agentSelector).toLowerCase().includes(q),
  )
})

// ── Selector label input ───────────────────────────────────────
const selectorInput = ref('')

function onOpenDrawer(type: 'view' | 'edit' | 'create', policy?: any) {
  store.openDrawer(type, policy)
  selectorInput.value = store.getSelectorLabel()
}

// ── Handlers ───────────────────────────────────────────────────
async function handleSave() {
  const ok = await store.handleCreateOrUpdate()
  if (ok) {
    toast.success(
      store.drawerType === 'create'
        ? t('manage.agentPolicies.createSuccess')
        : t('manage.agentPolicies.editSuccess'),
    )
  } else {
    toast.error(t('manage.agentPolicies.saveFailed'))
  }
}

async function handleDelete() {
  const ok = await store.handleDelete()
  if (ok) {
    toast.success(t('manage.agentPolicies.deleteSuccess'))
  } else {
    toast.error(t('manage.agentPolicies.deleteFailed'))
  }
}

// ── Lifecycle ──────────────────────────────────────────────────
onMounted(() => store.refresh())

// ── Stat card click filter ─────────────────────────────────────
const activeFilter = ref<string | null>(null)

function toggleFilter(key: string) {
  activeFilter.value = activeFilter.value === key ? null : key
}
</script>

<template>
  <div class="p-6 space-y-6">
    <!-- Stat cards -->
    <div class="grid grid-cols-3 gap-4">
      <div
        class="flex items-center gap-3 rounded-xl border border-border bg-card p-4 cursor-pointer transition-colors"
        :class="{ 'ring-2 ring-primary': activeFilter === 'all' }"
        @click="toggleFilter('all')"
      >
        <div class="flex size-10 items-center justify-center rounded-lg bg-primary/10">
          <ShieldCheck class="size-5 text-primary" />
        </div>
        <div>
          <p class="text-2xl font-bold">{{ store.stats.total }}</p>
          <p class="text-xs text-muted-foreground">{{ t('manage.agentPolicies.statTotal') }}</p>
        </div>
      </div>

      <div
        class="flex items-center gap-3 rounded-xl border border-border bg-card p-4 cursor-pointer transition-colors"
        :class="{ 'ring-2 ring-orange-500': activeFilter === 'deny' }"
        @click="toggleFilter('deny')"
      >
        <div class="flex size-10 items-center justify-center rounded-lg bg-orange-500/10">
          <ShieldAlert class="size-5 text-orange-500" />
        </div>
        <div>
          <p class="text-2xl font-bold">{{ store.stats.defaultDeny }}</p>
          <p class="text-xs text-muted-foreground">{{ t('manage.agentPolicies.statDeny') }}</p>
        </div>
      </div>

      <div
        class="flex items-center gap-3 rounded-xl border border-border bg-card p-4 cursor-pointer transition-colors"
        :class="{ 'ring-2 ring-emerald-500': activeFilter === 'allow' }"
        @click="toggleFilter('allow')"
      >
        <div class="flex size-10 items-center justify-center rounded-lg bg-emerald-500/10">
          <ShieldOff class="size-5 text-emerald-500" />
        </div>
        <div>
          <p class="text-2xl font-bold">{{ store.stats.allowAll }}</p>
          <p class="text-xs text-muted-foreground">{{ t('manage.agentPolicies.statAllow') }}</p>
        </div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center justify-between">
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          v-model="searchQuery"
          :placeholder="t('manage.agentPolicies.searchPlaceholder')"
          class="pl-9"
        />
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" size="sm" @click="store.refresh()">
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" @click="onOpenDrawer('create')">
          <Plus class="mr-1.5 size-4" />
          {{ t('manage.agentPolicies.create') }}
        </Button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="store.loading && store.rows.length === 0" class="space-y-2">
      <div v-for="i in 4" :key="i" class="h-12 animate-pulse rounded-lg bg-muted" />
    </div>

    <!-- Empty -->
    <div
      v-else-if="filteredRows.length === 0"
      class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground"
    >
      <ShieldCheck class="size-10 opacity-40" />
      <p>{{ t('manage.agentPolicies.empty') }}</p>
      <Button variant="outline" size="sm" class="mt-2" @click="onOpenDrawer('create')">
        <Plus class="mr-1.5 size-3.5" />{{ t('manage.agentPolicies.createFirst') }}
      </Button>
    </div>

    <!-- Table -->
    <div v-else class="rounded-xl border border-border bg-card overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead class="w-[180px]">{{ t('manage.agentPolicies.colName') }}</TableHead>
            <TableHead>{{ t('manage.agentPolicies.colSelector') }}</TableHead>
            <TableHead class="w-[100px]">{{ t('manage.agentPolicies.colDefaultDeny') }}</TableHead>
            <TableHead>{{ t('manage.agentPolicies.colAllowedTools') }}</TableHead>
            <TableHead class="w-[120px]">{{ t('manage.agentPolicies.colActions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="policy in filteredRows" :key="policy.name">
            <TableCell class="font-medium">{{ policy.name }}</TableCell>
            <TableCell class="font-mono text-xs text-muted-foreground">
              {{ store.selectorToString(policy.spec.agentSelector) }}
            </TableCell>
            <TableCell>
              <Badge
                :class="policy.spec.defaultDeny
                  ? 'bg-orange-500/10 text-orange-600 dark:text-orange-400'
                  : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'"
                variant="secondary"
              >
                {{ policy.spec.defaultDeny ? t('manage.agentPolicies.deny') : t('manage.agentPolicies.allow') }}
              </Badge>
            </TableCell>
            <TableCell class="text-xs text-muted-foreground max-w-[300px] truncate">
              {{ store.toolsSummary(policy.spec.allowedTools) }}
            </TableCell>
            <TableCell>
              <div class="flex items-center gap-1">
                <Button variant="ghost" size="icon" class="size-8" @click="onOpenDrawer('view', policy)">
                  <Eye class="size-4" />
                </Button>
                <Button variant="ghost" size="icon" class="size-8" @click="onOpenDrawer('edit', policy)">
                  <Pencil class="size-4" />
                </Button>
                <Button
                  variant="ghost" size="icon"
                  class="size-8 text-muted-foreground hover:text-destructive"
                  @click="store.openDeleteDialog(policy)"
                >
                  <Trash2 class="size-4" />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Create / Edit Dialog ─────────────────────────────────── -->
    <Dialog
      :open="store.isDrawerOpen"
      @update:open="(v: boolean) => { if (!v) store.isDrawerOpen = false }"
    >
      <DialogContent class="sm:max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {{ store.drawerType === 'create'
              ? t('manage.agentPolicies.createTitle')
              : store.drawerType === 'edit'
                ? t('manage.agentPolicies.editTitle')
                : t('manage.agentPolicies.viewTitle') }}
          </DialogTitle>
          <DialogDescription>
            {{ store.drawerType === 'view'
              ? t('manage.agentPolicies.viewDesc')
              : t('manage.agentPolicies.editDesc') }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-5">
          <!-- Name -->
          <div class="space-y-1.5">
            <Label>{{ t('manage.agentPolicies.formName') }}</Label>
            <Input
              v-model="store.formName"
              :placeholder="t('manage.agentPolicies.formNamePlaceholder')"
              :disabled="store.drawerType === 'view' || store.drawerType === 'edit'"
            />
          </div>

          <!-- Agent Selector -->
          <div class="space-y-1.5">
            <Label>{{ t('manage.agentPolicies.formSelector') }}</Label>
            <Input
              v-model="selectorInput"
              placeholder="agent-name=my-agent (empty = match all)"
              :disabled="store.drawerType === 'view'"
              @input="store.setSelectorLabel(selectorInput)"
            />
            <p class="text-xs text-muted-foreground">
              {{ t('manage.agentPolicies.formSelectorHelp') }}
            </p>
          </div>

          <!-- Default Deny toggle -->
          <div class="flex items-center justify-between rounded-lg border p-3">
            <div class="space-y-0.5">
              <Label>{{ t('manage.agentPolicies.formDefaultDeny') }}</Label>
              <p class="text-xs text-muted-foreground">
                {{ t('manage.agentPolicies.formDefaultDenyHelp') }}
              </p>
            </div>
            <Switch
              :checked="store.formSpec.defaultDeny"
              :disabled="store.drawerType === 'view'"
              @update:checked="(v: boolean) => store.formSpec.defaultDeny = v"
            />
          </div>

          <!-- Allowed Tools (only when defaultDeny) -->
          <div v-if="store.formSpec.defaultDeny" class="space-y-3">
            <div class="flex items-center justify-between">
              <Label>{{ t('manage.agentPolicies.formAllowedTools') }}</Label>
              <Button
                v-if="store.drawerType !== 'view'"
                variant="outline" size="sm"
                @click="store.addToolPermission()"
              >
                <Plus class="mr-1 size-3" />{{ t('manage.agentPolicies.addServer') }}
              </Button>
            </div>

            <div
              v-for="(perm, pi) in store.formSpec.allowedTools"
              :key="pi"
              class="rounded-lg border p-3 space-y-2"
            >
              <div class="flex items-center gap-2">
                <Input
                  v-model="perm.mcpServer"
                  :placeholder="t('manage.agentPolicies.formServerName')"
                  class="flex-1"
                  :disabled="store.drawerType === 'view'"
                />
                <Button
                  v-if="store.drawerType !== 'view'"
                  variant="ghost" size="icon" class="size-8 text-destructive shrink-0"
                  @click="store.removeToolPermission(pi)"
                >
                  <X class="size-4" />
                </Button>
              </div>

              <div class="space-y-1">
                <div
                  v-for="(_tool, ti) in perm.tools"
                  :key="ti"
                  class="flex items-center gap-2"
                >
                  <Input
                    v-model="perm.tools[ti]"
                    placeholder="* or tool_name"
                    class="flex-1 text-xs"
                    :disabled="store.drawerType === 'view'"
                  />
                  <Button
                    v-if="store.drawerType !== 'view' && perm.tools.length > 1"
                    variant="ghost" size="icon" class="size-6 shrink-0"
                    @click="store.removeToolFromPermission(pi, ti)"
                  >
                    <X class="size-3" />
                  </Button>
                </div>
                <Button
                  v-if="store.drawerType !== 'view'"
                  variant="ghost" size="sm" class="text-xs text-muted-foreground"
                  @click="store.addToolToPermission(pi)"
                >
                  <Plus class="mr-1 size-3" />{{ t('manage.agentPolicies.addTool') }}
                </Button>
              </div>
            </div>

            <p
              v-if="!store.formSpec.allowedTools?.length"
              class="text-xs text-muted-foreground text-center py-4"
            >
              {{ t('manage.agentPolicies.noToolsHint') }}
            </p>
          </div>
        </div>

        <DialogFooter v-if="store.drawerType !== 'view'">
          <Button variant="outline" @click="store.isDrawerOpen = false">
            {{ t('common.action.cancel') }}
          </Button>
          <Button :disabled="store.loading || !store.formName.trim()" @click="handleSave">
            <Loader2 v-if="store.loading" class="mr-2 size-4 animate-spin" />
            {{ t('common.action.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- ── Delete Confirmation ──────────────────────────────────── -->
    <Dialog
      :open="store.deleteDialogOpen"
      @update:open="(v: boolean) => { if (!v) store.deleteDialogOpen = false }"
    >
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('manage.agentPolicies.deleteTitle') }}</DialogTitle>
          <DialogDescription>
            {{ t('manage.agentPolicies.deleteDesc', { name: store.deleteTarget?.name }) }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="store.deleteDialogOpen = false">
            {{ t('common.action.cancel') }}
          </Button>
          <Button variant="destructive" :disabled="store.loading" @click="handleDelete">
            <Loader2 v-if="store.loading" class="mr-2 size-4 animate-spin" />
            {{ t('common.action.delete') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
