import { defineStore } from 'pinia'
import { ref, reactive, watch } from 'vue'
import { listWs, add, deleteWs } from "@/api/workspace"
import { useAction, useTable } from '@/composables/useApi'
import { useConfirm } from '@/composables/useConfirm'
import { useWorkspaceStore } from '@/stores/workspace'
import { useRouter } from 'vue-router'
import router from "@/router";

export const useWorkspacePageStore = defineStore('workspacePage', () => {
    const router = useRouter()
    const globalWsStore = useWorkspaceStore()
    const { confirm } = useConfirm()

    // =========================================================
    // 1. DATA (State) - 仅存放响应式数据
    // =========================================================
    const { rows, total, loading, params, refresh } = useTable(listWs, {
        successMsg: '列表已同步',
    })

    const ui = reactive({
        isDrawerOpen: false,
        drawerType: 'create' as 'view' | 'edit' | 'create',
        isSubmitting: false
    })

    const form = ref({
        displayName: '',
        slug: '',
        maxNodeCount: 20
    })

    // 内部监听分页同步
    watch(() => [params.page, params.pageSize], () => refresh(), { deep: true })

    // =========================================================
    // 2. ACTIONS (Methods) - 仅存放业务逻辑
    // =========================================================
    const { execute: runAdd } = useAction(add, {
        successMsg: "空间创建成功",
        onSuccess: () => {
            actions.closeDrawer()
            refresh()
        }
    })

    const actions = {
        // 刷新数据
        refresh,

        // 弹窗管理
        openDrawer(type: 'view' | 'edit' | 'create', data: any = null) {
            ui.drawerType = type
            if (data) {
                form.value = { ...data }
            } else {
                form.value = { displayName: '', slug: '', maxNodeCount: 20 }
            }
            ui.isDrawerOpen = true
        },

        closeDrawer() {
            ui.isDrawerOpen = false
        },

        // 核心业务
        async handleCreate() {
            if (!form.value.displayName) return
            ui.isSubmitting = true
            try {
                await runAdd(form.value)
            } finally {
                ui.isSubmitting = false
            }
        },

        async handleDelete(ws: any) {
            const isConfirmed = await confirm({
                title: '销毁确认',
                message: `确定要删除 ${ws.displayName} 吗？`,
                type: 'danger'
            })

            if (isConfirmed) {
                await deleteWs(ws.id)
                refresh()
            }
        },

        enterWorkspace(ws: any) {
            globalWsStore.switchWorkspace(ws)
            router.push(`/ws/${ws.id}/nodes`)
        }
    }

    // 返回时清晰地分为两类
    return {
        // Data/State
        rows,
        total,
        loading,
        params,
        ui,
        form,
        // Actions
        actions
    }
})