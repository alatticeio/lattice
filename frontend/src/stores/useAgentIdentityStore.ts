import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listAgentIdentities, createAgentIdentity, updateAgentIdentity, deleteAgentIdentity,
  type AgentIdentity, type AgentIdentityInput,
} from '@/api/agent-identity'

export const useAgentIdentityStore = defineStore('agentIdentity', () => {
  const rows = ref<AgentIdentity[]>([])
  const total = ref(0)
  const loading = ref(false)

  // ── Drawer state ──────────────────────────────────────────────
  const isDrawerOpen = ref(false)
  const drawerType = ref<'view' | 'edit' | 'create'>('view')
  const selectedIdentity = ref<AgentIdentity | null>(null)

  // ── Form state ────────────────────────────────────────────────
  const getEmptyForm = (): AgentIdentityInput => ({
    name: '',
    peer_ref: '',
    allowed_tools: [],
    allowed_namespaces: [],
    sandbox: 'none',
    audit_level: 'write',
    enforcement_mode: 'enforce',
    parent_ref: '',
    spawnable_roles: [],
    description: '',
  })

  const form = ref<AgentIdentityInput>(getEmptyForm())

  // ── Delete dialog ─────────────────────────────────────────────
  const deleteDialogOpen = ref(false)
  const deleteTarget = ref<AgentIdentity | null>(null)

  // ── Computed stats ────────────────────────────────────────────
  const stats = computed(() => {
    const all = rows.value
    return {
      total: all.length,
      active: all.filter(i => i.phase === 'Active').length,
      sandboxed: all.filter(i => i.sandbox && i.sandbox !== 'none').length,
    }
  })

  // ── Actions ───────────────────────────────────────────────────
  async function refresh() {
    loading.value = true
    try {
      const res = await listAgentIdentities()
      rows.value = res.data ?? []
      total.value = rows.value.length
    } catch {
      // toast handled by caller
    } finally {
      loading.value = false
    }
  }

  function parseJSON(s?: string): string[] {
    if (!s) return []
    try { return JSON.parse(s) } catch { return [] }
  }

  function openDrawer(type: 'view' | 'edit' | 'create', identity?: AgentIdentity) {
    drawerType.value = type
    if (type === 'create') {
      form.value = getEmptyForm()
      selectedIdentity.value = null
    } else if (identity) {
      form.value = {
        name: identity.name,
        peer_ref: identity.peer_ref,
        allowed_tools: parseJSON(identity.allowed_tools),
        allowed_namespaces: parseJSON(identity.allowed_namespaces),
        sandbox: identity.sandbox,
        audit_level: identity.audit_level,
        enforcement_mode: identity.enforcement_mode,
        parent_ref: identity.parent_ref || '',
        spawnable_roles: parseJSON(identity.spawnable_roles),
        description: identity.description || '',
      }
      selectedIdentity.value = identity
    }
    isDrawerOpen.value = true
  }

  async function handleCreateOrUpdate(): Promise<boolean> {
    if (!form.value.name.trim() || !form.value.peer_ref.trim()) return false
    loading.value = true
    try {
      if (drawerType.value === 'create') {
        await createAgentIdentity(form.value)
      } else if (selectedIdentity.value) {
        await updateAgentIdentity(selectedIdentity.value.id, form.value)
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

  async function handleDelete(): Promise<boolean> {
    if (!deleteTarget.value) return false
    loading.value = true
    try {
      await deleteAgentIdentity(deleteTarget.value.id)
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

  function openDeleteDialog(identity: AgentIdentity) {
    deleteTarget.value = identity
    deleteDialogOpen.value = true
  }

  return {
    rows, total, loading, stats,
    isDrawerOpen, drawerType, selectedIdentity,
    form, parseJSON,
    deleteDialogOpen, deleteTarget,
    refresh, openDrawer,
    handleCreateOrUpdate, handleDelete, openDeleteDialog,
  }
})
