import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listPeerIdentities, createPeerIdentity, updatePeerIdentity, deletePeerIdentity,
  type PeerIdentity, type PeerIdentityInput,
} from '@/api/peer-identity'

export const usePeerIdentityStore = defineStore('peerIdentity', () => {
  const rows = ref<PeerIdentity[]>([])
  const total = ref(0)
  const loading = ref(false)

  // ── Drawer state ──────────────────────────────────────────────
  const isDrawerOpen = ref(false)
  const drawerType = ref<'view' | 'edit' | 'create'>('view')
  const selectedIdentity = ref<PeerIdentity | null>(null)

  // ── Form state ────────────────────────────────────────────────
  const getEmptyForm = (): PeerIdentityInput => ({
    name: '',
    peer_ref: '',
    previous_peer_ref: '',
    grace_period_seconds: 300,
    description: '',
  })

  const form = ref<PeerIdentityInput>(getEmptyForm())

  // ── Delete dialog ─────────────────────────────────────────────
  const deleteDialogOpen = ref(false)
  const deleteTarget = ref<PeerIdentity | null>(null)

  // ── Computed stats ────────────────────────────────────────────
  const stats = computed(() => {
    const all = rows.value
    const now = new Date()
    return {
      total: all.length,
      bound: all.filter(i => i.resolved_peer_ip).length,
      gracePeriod: all.filter(i =>
        i.grace_period_expires_at && new Date(i.grace_period_expires_at) > now,
      ).length,
    }
  })

  // ── Actions ───────────────────────────────────────────────────
  async function refresh() {
    loading.value = true
    try {
      const res = await listPeerIdentities()
      rows.value = res.data ?? []
      total.value = rows.value.length
    } catch {
      // toast handled by caller
    } finally {
      loading.value = false
    }
  }

  function openDrawer(type: 'view' | 'edit' | 'create', identity?: PeerIdentity) {
    drawerType.value = type
    if (type === 'create') {
      form.value = getEmptyForm()
      selectedIdentity.value = null
    } else if (identity) {
      form.value = {
        name: identity.name,
        peer_ref: identity.peer_ref,
        previous_peer_ref: identity.previous_peer_ref || '',
        grace_period_seconds: identity.grace_period_seconds,
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
        await createPeerIdentity(form.value)
      } else if (selectedIdentity.value) {
        await updatePeerIdentity(selectedIdentity.value.id, form.value)
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
      await deletePeerIdentity(deleteTarget.value.id)
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

  function openDeleteDialog(identity: PeerIdentity) {
    deleteTarget.value = identity
    deleteDialogOpen.value = true
  }

  return {
    rows, total, loading, stats,
    isDrawerOpen, drawerType, selectedIdentity,
    form,
    deleteDialogOpen, deleteTarget,
    refresh, openDrawer,
    handleCreateOrUpdate, handleDelete, openDeleteDialog,
  }
})
