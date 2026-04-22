import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { listPeer, updatePeer } from '@/api/user'
import { useAction, useTable } from '@/composables/useApi'
import { toast } from 'vue-sonner'

export const usePeerPageStore = defineStore('peerPage', () => {

    // ── State ──────────────────────────────────────────────────────
    const { rows, total, loading, params, refresh } = useTable(listPeer, {
        successMsg: '刷新列表成功'
    })

    const isDrawerOpen  = ref(false)
    const drawerType    = ref<'view' | 'edit'>('view')
    const newLabelInput = ref('')
    const isUpdating    = ref(false)

    const selectedNode = ref<{
        appId: string
        name?: string
        publicKey: string
        region?: string
        namespace?: string
        workspaceDisplayName?: string
        address?: string
        network?: string
        status?: string
        lastSeen?: string
        labels: string[]
    }>({
        appId: '',
        publicKey: '',
        region: '',
        namespace: '',
        labels: []
    })

    // ── Watch pagination ───────────────────────────────────────────
    watch(() => [params.page, params.pageSize], () => refresh(), { deep: true })
    watch(() => params.search, () => {
        params.page = 1
        refresh()
    })

    // ── Update action ──────────────────────────────────────────────
    const { execute: runUpdate } = useAction(updatePeer, {
        successMsg: '节点信息已同步',
        onSuccess: () => {
            isDrawerOpen.value = false
            refresh()
        }
    })

    // ── Actions ────────────────────────────────────────────────────
    const actions = {
        refresh,

        openDrawer(type: 'view' | 'edit', node: any) {
            drawerType.value = type

            // labels: Map → Array (key=value)
            const formattedLabels: string[] = []
            if (node.labels && typeof node.labels === 'object' && !Array.isArray(node.labels)) {
                Object.entries(node.labels).forEach(([k, v]) => {
                    formattedLabels.push(`${k}=${v}`)
                })
            } else if (Array.isArray(node.labels)) {
                formattedLabels.push(...node.labels)
            }

            selectedNode.value = {
                ...JSON.parse(JSON.stringify(node)),
                labels: formattedLabels
            }
            isDrawerOpen.value = true
        },

        addLabel() {
            const val = newLabelInput.value.trim()
            if (!val) return
            if (val.includes('=')) {
                const inputKey = val.split('=')[0].trim()
                const idx = selectedNode.value.labels.findIndex(l => l.split('=')[0].trim() === inputKey)
                if (idx !== -1) selectedNode.value.labels[idx] = val
                else selectedNode.value.labels.push(val)
            } else {
                if (!selectedNode.value.labels.includes(val)) selectedNode.value.labels.push(val)
            }
            newLabelInput.value = ''
        },

        removeLabel(index: number) {
            selectedNode.value.labels.splice(index, 1)
        },

        async handleSave() {
            isUpdating.value = true
            try {
                const labelMap: Record<string, string> = {}
                selectedNode.value.labels.forEach(item => {
                    if (item.includes('=')) {
                        const [key, ...val] = item.split('=')
                        labelMap[key.trim()] = val.join('=').trim()
                    } else {
                        labelMap[item.trim()] = 'true'
                    }
                })
                await runUpdate({ ...selectedNode.value, labels: labelMap })
            } finally {
                isUpdating.value = false
            }
        },

        async handleDelete(_node: any, confirmFn: () => Promise<boolean>) {
            const ok = await confirmFn()
            if (!ok) return
            loading.value = true
            try {
                // TODO: deletePeer API not yet implemented
                toast('删除成功')
                refresh()
            } finally {
                loading.value = false
            }
        },
    }

    return {
        rows, total, loading, params,
        isDrawerOpen, drawerType, newLabelInput, isUpdating,
        selectedNode, actions,
    }
})
