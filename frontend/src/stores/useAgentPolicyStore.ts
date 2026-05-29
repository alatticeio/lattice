import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listAgentPolicies, createAgentPolicy, updateAgentPolicy, deleteAgentPolicy,
  type AgentPolicy, type AgentPolicySpec, type AgentToolPermission,
} from '@/api/agent-policy'

export const useAgentPolicyStore = defineStore('agentPolicy', () => {
  const rows = ref<AgentPolicy[]>([])
  const total = ref(0)
  const loading = ref(false)

  // ── Drawer state ──────────────────────────────────────────────
  const isDrawerOpen = ref(false)
  const drawerType = ref<'view' | 'edit' | 'create'>('view')
  const selectedPolicy = ref<AgentPolicy | null>(null)

  // ── Form state ────────────────────────────────────────────────
  const getEmptySpec = (): AgentPolicySpec => ({
    agentSelector: { matchLabels: {} },
    allowedTools: [],
    defaultDeny: true,
  })

  const formName = ref('')
  const formSpec = ref<AgentPolicySpec>(getEmptySpec())

  // ── Delete dialog ─────────────────────────────────────────────
  const deleteDialogOpen = ref(false)
  const deleteTarget = ref<AgentPolicy | null>(null)

  // ── Computed stats ────────────────────────────────────────────
  const stats = computed(() => {
    const all = rows.value
    return {
      total: all.length,
      defaultDeny: all.filter(p => p.spec.defaultDeny).length,
      allowAll: all.filter(p => !p.spec.defaultDeny).length,
    }
  })

  // ── Actions ───────────────────────────────────────────────────
  async function refresh() {
    loading.value = true
    try {
      const res = await listAgentPolicies()
      rows.value = res.data ?? []
      total.value = rows.value.length
    } catch {
      // toast handled by caller
    } finally {
      loading.value = false
    }
  }

  function openDrawer(type: 'view' | 'edit' | 'create', policy?: AgentPolicy) {
    drawerType.value = type
    if (type === 'create') {
      formName.value = ''
      formSpec.value = getEmptySpec()
      selectedPolicy.value = null
    } else if (policy) {
      formName.value = policy.name
      formSpec.value = JSON.parse(JSON.stringify(policy.spec))
      selectedPolicy.value = policy
    }
    isDrawerOpen.value = true
  }

  function addToolPermission() {
    if (!formSpec.value.allowedTools) formSpec.value.allowedTools = []
    formSpec.value.allowedTools.push({ mcpServer: '', tools: ['*'] })
  }

  function removeToolPermission(index: number) {
    formSpec.value.allowedTools?.splice(index, 1)
  }

  function addToolToPermission(permIndex: number) {
    formSpec.value.allowedTools?.[permIndex]?.tools.push('')
  }

  function removeToolFromPermission(permIndex: number, toolIndex: number) {
    formSpec.value.allowedTools?.[permIndex]?.tools.splice(toolIndex, 1)
  }

  async function handleCreateOrUpdate() {
    if (!formName.value.trim()) return
    loading.value = true
    try {
      if (drawerType.value === 'create') {
        await createAgentPolicy({ name: formName.value.trim(), spec: formSpec.value })
      } else {
        await updateAgentPolicy(formName.value, formSpec.value)
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
    if (!deleteTarget.value) return
    loading.value = true
    try {
      await deleteAgentPolicy(deleteTarget.value.name)
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

  function openDeleteDialog(policy: AgentPolicy) {
    deleteTarget.value = policy
    deleteDialogOpen.value = true
  }

  // ── Selector helpers ──────────────────────────────────────────
  function setSelectorLabel(str: string) {
    if (!str || !str.includes('=')) {
      formSpec.value.agentSelector.matchLabels = {}
      return
    }
    const [k, v] = str.split('=')
    formSpec.value.agentSelector.matchLabels = { [k.trim()]: v.trim() }
  }

  function getSelectorLabel(): string {
    const labels = formSpec.value.agentSelector.matchLabels
    const keys = Object.keys(labels)
    if (keys.length === 0) return ''
    return `${keys[0]}=${labels[keys[0]]}`
  }

  function selectorToString(sel: { matchLabels: Record<string, string> }): string {
    const keys = Object.keys(sel.matchLabels)
    if (keys.length === 0) return '*'
    return keys.map(k => `${k}=${sel.matchLabels[k]}`).join(', ')
  }

  function toolsSummary(perms?: AgentToolPermission[]): string {
    if (!perms || perms.length === 0) return '-'
    return perms.map(p => {
      const toolStr = p.tools.includes('*') ? '*' : p.tools.join(', ')
      return `${p.mcpServer}: ${toolStr}`
    }).join('; ')
  }

  return {
    rows, total, loading, stats,
    isDrawerOpen, drawerType, selectedPolicy,
    formName, formSpec,
    deleteDialogOpen, deleteTarget,
    refresh, openDrawer,
    addToolPermission, removeToolPermission,
    addToolToPermission, removeToolFromPermission,
    handleCreateOrUpdate, handleDelete, openDeleteDialog,
    setSelectorLabel, getSelectorLabel, selectorToString, toolsSummary,
  }
})
