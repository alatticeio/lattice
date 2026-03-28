import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { listWs } from "@/api/workspace"
import { useTable } from '@/composables/useApi'
import { useUserStore } from '@/stores/user'
import { useWorkspaceStore } from "@/stores/workspace"

export const useNavbarStore = defineStore('navbarPage', () => {
    const route = useRoute()
    const router = useRouter()
    const userStore = useUserStore()
    const wsStore = useWorkspaceStore()

    // =========================================================
    // 1. DATA (State)
    // =========================================================

    // 空间列表数据流
    const { rows, loading, refresh } = useTable(listWs, {
        successMsg: '空间同步成功',
        immediate: true
    })

    const searchQuery = ref('')

    // =========================================================
    // 2. COMPUTED (Getters)
    // =========================================================

    const currentWsId = computed(() => route.params.wsId as string)
    const userInfo = computed(() => userStore.userInfo)
    const currentWorkspace = computed(() => wsStore.currentWorkspace)

    // =========================================================
    // 3. WATCHERS (Sync Logic)
    // =========================================================

    // 核心逻辑：路由变化时，自动从列表里挑出对应的空间对象存入全局 wsStore
    watch(
        [() => route.params.wsId, rows],
        ([newId, newRows]) => {
            if (newId && newRows.length > 0) {
                const active = newRows.find((item:any) => item.id === newId)
                if (active) wsStore.switchWorkspace(active)
            }
        },
        { immediate: true, deep: true }
    )

    // =========================================================
    // 4. ACTIONS (Methods)
    // =========================================================
    const actions = {
        refresh,

        handleLogout() {
            localStorage.removeItem('wf_user')
            localStorage.removeItem('wf_token')
            router.push('/login')
        },

        switchWorkspace(ws: any) {
            wsStore.switchWorkspace(ws)
            // 如果需要跳转到该空间的首页
            router.push(`/ws/${ws.id}/nodes`)
        },

        handleSearch() {
            console.log('Searching for:', searchQuery.value)
            // 实现搜索跳转逻辑
        }
    }

    return {
        rows,
        loading,
        searchQuery,
        currentWsId,
        userInfo,
        currentWorkspace,
        actions
    }
})