import { defineStore } from 'pinia'
import { ref, reactive, watch, computed } from 'vue'
import { listPeer, updatePeer } from '@/api/user'
import { useAction, useTable } from '@/composables/useApi'
import { useConfirm } from '@/composables/useConfirm'

type DrawerMode = 'view' | 'edit' | 'create';

export const usePeerPageStore = defineStore('peerPage', () => {


    const { confirm } = useConfirm()

    // =========================================================
    // 1. DATA (State)
    // =========================================================

    // 列表数据
    const { rows, total, loading, params, refresh } = useTable(listPeer, {
        successMsg: '刷新列表成功',
        errorMsg: '刷新列表失败',
        initialParams: { namespace: 'wf-test' }
    })

    // UI 状态
    const ui = reactive({
        isDrawerOpen: false,
        drawerType: 'view' as DrawerMode, // 关键点：显式指定联合类型
        newLabelInput: '',
        isUpdating: false
    })

    // 当前选中的编辑节点
    const selectedNode = ref({
        appId: '',
        publicKey: '',
        region: '',
        namespace: '',
        labels: [] as string[] // 前端统一使用数组格式
    })

    // 监听分页与搜索
    watch(() => [params.page, params.pageSize], () => refresh(), { deep: true })
    watch(() => params.search, () => {
        params.page = 1
        refresh()
    })

    // =========================================================
    // 2. ACTIONS (Logic)
    // =========================================================

    const { execute: runUpdate } = useAction(updatePeer, {
        successMsg: '节点信息已同步',
        onSuccess: () => {
            ui.isDrawerOpen = false
            refresh()
        }
    })

    const actions = {
        refresh,

        // 打开详情/编辑
        openDrawer(type: 'view' | 'edit', node: any) {
            ui.drawerType = type

            // 数据转换：Map -> Array (用于前端展示和编辑)
            const formattedLabels: string[] = []
            if (node.labels && typeof node.labels === 'object') {
                Object.entries(node.labels).forEach(([k, v]) => {
                    formattedLabels.push(`${k}=${v}`)
                })
            }

            selectedNode.value = {
                ...JSON.parse(JSON.stringify(node)), // 深拷贝
                labels: formattedLabels
            }
            ui.isDrawerOpen = true
        },

        // 标签管理逻辑
        addLabel() {
            const val = ui.newLabelInput.trim()
            if (!val) return

            if (val.includes('=')) {
                const inputKey = val.split('=')[0].trim()
                const existingIndex = selectedNode.value.labels.findIndex(item => item.split('=')[0].trim() === inputKey)

                if (existingIndex !== -1) {
                    selectedNode.value.labels[existingIndex] = val
                } else {
                    selectedNode.value.labels.push(val)
                }
            } else {
                if (!selectedNode.value.labels.includes(val)) {
                    selectedNode.value.labels.push(val)
                }
            }
            ui.newLabelInput = ''
        },

        getPeers() {

        },

        removeLabel(index: number) {
            selectedNode.value.labels.splice(index, 1)
        },

        // 保存逻辑
        async handleSave() {
            ui.isUpdating = true
            try {
                // 数据还原：Array -> Map (还原回后端格式)
                const labelMap: Record<string, string> = {}
                selectedNode.value.labels.forEach(item => {
                    if (item.includes('=')) {
                        const [key, ...val] = item.split('=')
                        labelMap[key.trim()] = val.join('=').trim()
                    } else {
                        labelMap[item.trim()] = "true"
                    }
                })

                const payload = {
                    ...selectedNode.value,
                    labels: labelMap,
                    namespace: 'wf-test'
                }
                await runUpdate(payload)
            } finally {
                ui.isUpdating = false
            }
        },

        // 删除逻辑
        async handleDelete(node: any) {
            const isConfirmed = await confirm({
                title: '确认删除节点？',
                message: `你正在尝试删除节点 <span class="text-error font-bold">${node.appId}</span>。`,
                confirmText: '立即销毁',
                type: 'danger'
            })

            if (isConfirmed) {
                // await deletePeer(node.appId)
                refresh()
            }
        }
    }

    return {
        rows, total, loading, params, ui, selectedNode,
        actions
    }
})