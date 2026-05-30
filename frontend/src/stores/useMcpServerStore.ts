import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listMCPServers, createMCPServer, updateMCPServer, deleteMCPServer, discoverMCPTools,
  type MCPServer, type MCPServerSpec, type MCPTool,
} from '@/api/mcp-server'

export const useMcpServerStore = defineStore('mcpServer', () => {
  const rows = ref<MCPServer[]>([])
  const total = ref(0)
  const loading = ref(false)

  // ── Drawer state ──────────────────────────────────────────────
  const isDrawerOpen = ref(false)
  const drawerType = ref<'view' | 'edit' | 'create'>('view')
  const selectedServer = ref<MCPServer | null>(null)

  // ── Form state ────────────────────────────────────────────────
  const getEmptySpec = (): MCPServerSpec => ({
    peerName: '',
    endpoint: '',
    tools: [],
  })

  const formName = ref('')
  const formSpec = ref<MCPServerSpec>(getEmptySpec())

  // ── Delete dialog ─────────────────────────────────────────────
  const deleteDialogOpen = ref(false)
  const deleteTarget = ref<MCPServer | null>(null)

  // ── Computed stats ────────────────────────────────────────────
  const stats = computed(() => {
    const all = rows.value
    return {
      total: all.length,
      ready: all.filter(s => s.status?.phase === 'Ready').length,
      internal: all.filter(s => s.status?.mode === 'internal').length,
    }
  })

  // ── Actions ───────────────────────────────────────────────────
  async function refresh() {
    loading.value = true
    try {
      const res = await listMCPServers()
      rows.value = res.data ?? []
      total.value = rows.value.length
    } catch {
      // toast handled by caller
    } finally {
      loading.value = false
    }
  }

  function openDrawer(type: 'view' | 'edit' | 'create', server?: MCPServer) {
    drawerType.value = type
    if (type === 'create') {
      formName.value = ''
      formSpec.value = getEmptySpec()
      selectedServer.value = null
    } else if (server) {
      formName.value = server.name
      formSpec.value = JSON.parse(JSON.stringify(server.spec))
      selectedServer.value = server
    }
    isDrawerOpen.value = true
  }

  function addTool() {
    if (!formSpec.value.tools) formSpec.value.tools = []
    formSpec.value.tools.push({ name: '', description: '', riskLevel: 'low' })
  }

  function removeTool(index: number) {
    formSpec.value.tools?.splice(index, 1)
  }

  // ── Discover tools from MCP server ───────────────────────────────
  const discovering = ref(false)

  async function discoverTools(): Promise<boolean> {
    const endpoint = formSpec.value.endpoint?.trim()
    if (!endpoint) return false
    discovering.value = true
    try {
      const res = await discoverMCPTools(endpoint)
      if (res.data?.length) {
        formSpec.value.tools = res.data
        return true
      }
      return false
    } catch {
      return false
    } finally {
      discovering.value = false
    }
  }

  async function handleCreateOrUpdate() {
    if (!formName.value.trim()) return false
    loading.value = true
    try {
      if (drawerType.value === 'create') {
        await createMCPServer({ name: formName.value.trim(), spec: formSpec.value })
      } else {
        await updateMCPServer(formName.value, formSpec.value)
      }
      isDrawerOpen.value = false
      await refresh()
      return true
    } catch {
      return false
    } finally {
      loading.value = false
    }
  }

  async function handleDelete() {
    if (!deleteTarget.value) return false
    loading.value = true
    try {
      await deleteMCPServer(deleteTarget.value.name)
      deleteDialogOpen.value = false
      deleteTarget.value = null
      await refresh()
      return true
    } catch {
      return false
    } finally {
      loading.value = false
    }
  }

  function openDeleteDialog(server: MCPServer) {
    deleteTarget.value = server
    deleteDialogOpen.value = true
  }

  // ── Tool helpers ──────────────────────────────────────────────
  function toolsSummary(tools?: MCPTool[]): string {
    if (!tools || tools.length === 0) return '-'
    return tools.map(t => t.name || '(unnamed)').join(', ')
  }

  function phaseClass(phase?: string): string {
    const map: Record<string, string> = {
      Ready: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
      Pending: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
      Degraded: 'bg-red-500/10 text-red-600 dark:text-red-400',
    }
    return map[phase ?? ''] ?? 'bg-muted text-muted-foreground'
  }

  function modeClass(mode?: string): string {
    const map: Record<string, string> = {
      internal: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
      external: 'bg-violet-500/10 text-violet-600 dark:text-violet-400',
    }
    return map[mode ?? ''] ?? 'bg-muted text-muted-foreground'
  }

  return {
    rows, total, loading, stats, discovering,
    isDrawerOpen, drawerType, selectedServer,
    formName, formSpec,
    deleteDialogOpen, deleteTarget,
    refresh, openDrawer,
    addTool, removeTool, discoverTools,
    handleCreateOrUpdate, handleDelete, openDeleteDialog,
    toolsSummary, phaseClass, modeClass,
  }
})
