<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { ChevronsUpDown, Plus, Search, Check } from 'lucide-vue-next'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar,
} from '@/components/ui/sidebar'
import { Input } from '@/components/ui/input'
import { useWorkspaceStore, getWsInitials } from '@/stores/workspace'
import { getWsColor } from '@/utils/color'
import type { Workspace } from '@/stores/workspace'
import AddWorkspaceDialog from '@/components/app-sidebar/AddWorkspaceDialog.vue'

const { isMobile } = useSidebar()
const store = useWorkspaceStore()
const { currentWorkspace, allRows, allLoading } = storeToRefs(store)
const showAddDialog = ref(false)
const searchQuery = ref('')

onMounted(() => store.fetchAll())

const filtered = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return allRows.value
  return allRows.value.filter(ws => {
    const name = ws.displayName?.toLowerCase() ?? ''
    const slug = ws.slug?.toLowerCase() ?? ''
    return name.includes(q) || slug.includes(q)
  })
})

const showSearch = computed(() => allRows.value.length > 5)

function switchTo(ws: Workspace) {
  store.switchWorkspace(ws)
  searchQuery.value = ''
}

function onOpenChange(open: boolean) {
  if (!open) searchQuery.value = ''
}
</script>

<template>
  <SidebarMenu>
    <SidebarMenuItem>
      <DropdownMenu @update:open="onOpenChange">
        <DropdownMenuTrigger as-child>
          <SidebarMenuButton
            size="lg"
            class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          >
            <div
              class="flex aspect-square size-8 items-center justify-center rounded-lg text-xs font-bold shrink-0 transition-colors duration-200"
              :class="currentWorkspace ? getWsColor(currentWorkspace.displayName) : 'bg-muted-foreground/20 text-muted-foreground'"
            >
              {{ currentWorkspace ? getWsInitials(currentWorkspace.displayName) : '?' }}
            </div>
            <div class="grid flex-1 text-left text-sm leading-tight">
              <span class="truncate font-semibold">
                {{ currentWorkspace?.displayName ?? '选择工作空间' }}
              </span>
              <span class="truncate text-xs text-sidebar-foreground/50">
                {{ currentWorkspace ? (currentWorkspace.slug ?? currentWorkspace.namespace ?? currentWorkspace.id) : '请选择一个空间' }}
              </span>
            </div>
            <ChevronsUpDown class="ml-auto size-4 text-sidebar-foreground/50 shrink-0" />
          </SidebarMenuButton>
        </DropdownMenuTrigger>

        <DropdownMenuContent
          class="rounded-lg p-0 overflow-hidden"
          :style="{ width: 'var(--reka-dropdown-menu-trigger-width)', minWidth: '240px' }"
          align="start"
          :side="isMobile ? 'bottom' : 'right'"
          :side-offset="4"
        >
          <DropdownMenuLabel class="text-xs text-muted-foreground px-3 pt-2.5 pb-1.5">
            工作空间
          </DropdownMenuLabel>

          <!-- Search -->
          <div v-if="showSearch" class="px-2 pb-1.5">
            <div class="relative">
              <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground pointer-events-none" />
              <Input
                v-model="searchQuery"
                placeholder="搜索名称或 slug..."
                class="h-7 pl-8 text-xs border-muted"
                @keydown.stop
                @click.stop
              />
            </div>
          </div>
          <DropdownMenuSeparator v-if="showSearch" class="my-0" />

          <!-- Workspace list -->
          <div class="max-h-[280px] overflow-y-auto py-1">
            <template v-if="filtered.length">
              <DropdownMenuItem
                v-for="ws in filtered"
                :key="ws.id"
                class="gap-2.5 px-2 py-1.5 cursor-pointer"
                :class="ws.id === currentWorkspace?.id ? 'bg-accent/50' : ''"
                @select="switchTo(ws)"
              >
                <div
                  class="flex size-6 items-center justify-center rounded-sm text-[10px] font-bold shrink-0"
                  :class="getWsColor(ws.displayName)"
                >
                  {{ getWsInitials(ws.displayName) }}
                </div>

                <div class="flex flex-col flex-1 min-w-0">
                  <span class="truncate text-sm font-medium leading-snug">{{ ws.displayName }}</span>
                  <span class="truncate text-[11px] text-muted-foreground font-mono">{{ ws.slug }}</span>
                </div>

                <div class="flex items-center gap-1.5 shrink-0">
                  <span
                    class="size-1.5 rounded-full"
                    :class="ws.status === 'active' ? 'bg-emerald-500' : 'bg-muted-foreground/30'"
                  />
                  <Check v-if="ws.id === currentWorkspace?.id" class="size-3.5 text-primary" />
                  <span v-else class="size-3.5" />
                </div>
              </DropdownMenuItem>
            </template>
            <div v-else class="px-3 py-4 text-center text-xs text-muted-foreground">
              {{ allLoading ? '加载中...' : '未找到匹配的空间' }}
            </div>
          </div>

          <DropdownMenuSeparator class="my-0" />

          <!-- Create new -->
          <div class="py-1">
            <DropdownMenuItem class="gap-2.5 px-2 py-1.5 cursor-pointer" @select="showAddDialog = true">
              <div class="flex size-6 items-center justify-center rounded-sm border border-dashed border-muted-foreground/40 shrink-0">
                <Plus class="size-3.5 text-muted-foreground" />
              </div>
              <span class="text-sm text-muted-foreground font-medium">新建工作空间</span>
            </DropdownMenuItem>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
    </SidebarMenuItem>
  </SidebarMenu>

  <AddWorkspaceDialog
    v-model:open="showAddDialog"
    @created="store.fetchAll()"
  />
</template>
