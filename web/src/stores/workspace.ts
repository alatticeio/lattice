import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import { add, listWs, updateWs, deleteWs } from '@/api/workspace'
import type { Workspace, ListWsParams } from '@/api/workspace'

export type { Workspace }

// ── Shared display helpers (exported for use across components) ───────────────
// Uses chart-1…chart-5 CSS variables so colors follow the global theme.
// Pattern: tinted bg + matching foreground — same as status badges, always readable.


export function getWsInitials(name: string): string {
    const parts = name.trim().split(/[\s\-_]+/)
    if (parts.length >= 2 && parts[0] && parts[1])
        return (parts[0][0] + parts[1][0]).toUpperCase()
    return name.slice(0, 2).toUpperCase()
}

// ── Store ─────────────────────────────────────────────────────────────────────

export const useWorkspaceStore = defineStore('workspace', () => {
    const route = useRoute()

    // ── State ────────────────────────────────────────────────────────
    const currentWorkspace = ref<Workspace | null>(
        JSON.parse(localStorage.getItem('active_ws') || 'null')
    )

    // Paginated list — used by the workspace management page
    const rows     = ref<Workspace[]>([])
    const total    = ref(0)
    const page     = ref(1)
    const pageSize = ref(10)
    const loading  = ref(false)
    const saving   = ref(false)

    // Full list — used by the sidebar switcher (no pagination)
    const allRows      = ref<Workspace[]>([])
    const allLoading   = ref(false)

    // ── Getters ──────────────────────────────────────────────────────
    const activeId = computed(() => ((route?.params as any)?.wsId as string) || '')

    // ── Actions ──────────────────────────────────────────────────────

    /** 拉取工作空间列表（后端分页，供管理页使用） */
    async function fetchList(params?: ListWsParams) {
        loading.value = true
        if (params?.page)     page.value     = params.page
        if (params?.pageSize) pageSize.value = params.pageSize
        try {
            const { data, code } = await listWs({
                page: page.value,
                pageSize: pageSize.value,
                ...params,
            })
            if (code === 200) {
                rows.value  = Array.isArray(data) ? data : (data?.list ?? data?.items ?? data?.data ?? [])
                total.value = Array.isArray(data) ? rows.value.length : (data?.total ?? rows.value.length)
            }
        } catch {
            toast.error('获取工作空间列表失败')
        } finally {
            loading.value = false
        }
    }

    /** 拉取全量工作空间（不分页，供侧边栏切换器使用） */
    async function fetchAll() {
        allLoading.value = true
        try {
            const { data, code } = await listWs({ pageSize: 9999 })
            if (code === 200) {
                allRows.value = Array.isArray(data) ? data : (data?.list ?? data?.items ?? data?.data ?? [])
            }
        } catch {
            // silently fail — switcher will just be empty
        } finally {
            allLoading.value = false
        }
    }

    /**
     * 保存工作空间（新建 / 编辑统一入口）
     * @param form        表单数据
     * @param editingId   传入则为编辑，否则为新建
     * @returns           是否成功
     */
    async function saveWorkspace(form: Partial<Workspace>, editingId?: string): Promise<boolean> {
        saving.value = true
        try {
            if (editingId) {
                const { code } = await updateWs(editingId, form)
                if (code === 200) {
                    // Sync paginated list
                    const idx = rows.value.findIndex(w => w.id === editingId)
                    if (idx !== -1) rows.value[idx] = { ...rows.value[idx], ...form }
                    // Sync full list
                    const allIdx = allRows.value.findIndex(w => w.id === editingId)
                    if (allIdx !== -1) allRows.value[allIdx] = { ...allRows.value[allIdx], ...form }
                    // Sync active workspace if it's the one being edited
                    if (currentWorkspace.value?.id === editingId) {
                        currentWorkspace.value = { ...currentWorkspace.value, ...form }
                        localStorage.setItem('active_ws', JSON.stringify(currentWorkspace.value))
                    }
                    toast.success('工作空间已更新')
                    return true
                }
            } else {
                const { data, code } = await add(form)
                if (code === 200) {
                    if (data) {
                        rows.value.push(data)
                        allRows.value.push(data)
                    }
                    toast.success('工作空间已创建')
                    return true
                }
            }
            return false
        } catch {
            toast.error(editingId ? '更新失败' : '创建失败')
            return false
        } finally {
            saving.value = false
        }
    }

    /** 删除工作空间 */
    async function deleteWorkspace(id: string): Promise<boolean> {
        try {
            const { code } = await deleteWs(id)
            if (code === 200) {
                rows.value    = rows.value.filter(w => w.id !== id)
                allRows.value = allRows.value.filter(w => w.id !== id)
                if (currentWorkspace.value?.id === id) clear()
                toast.success('工作空间已删除')
                return true
            }
            return false
        } catch {
            toast.error('删除失败')
            return false
        }
    }

    /** 切换当前激活空间 */
    function switchWorkspace(ws: Workspace) {
        currentWorkspace.value = ws
        localStorage.setItem('active_ws', JSON.stringify(ws))
        localStorage.setItem('active_ws_id', ws.id)
    }

    /** 直接替换列表（兼容旧调用） */
    function setRows(data: Workspace[]) {
        rows.value = data
    }

    /** 清除激活状态（退出登录时调用） */
    function clear() {
        currentWorkspace.value = null
        localStorage.removeItem('active_ws')
        localStorage.removeItem('active_ws_id')
    }

    // 路由变化时同步 currentWorkspace（从 allRows 或 rows 中查找）
    watch(() => (route.params as any).wsId, (newId) => {
        if (newId && newId !== currentWorkspace.value?.id) {
            const found = allRows.value.find(w => w.id === newId)
                       ?? rows.value.find(w => w.id === newId)
            if (found) currentWorkspace.value = found
        }
    }, { immediate: true })

    return {
        currentWorkspace,
        activeId,
        rows,
        total,
        page,
        pageSize,
        loading,
        saving,
        allRows,
        allLoading,
        fetchList,
        fetchAll,
        saveWorkspace,
        deleteWorkspace,
        switchWorkspace,
        setRows,
        clear,
    }
})
