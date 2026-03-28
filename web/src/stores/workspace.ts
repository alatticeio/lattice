import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import {useRoute, useRouter} from 'vue-router'
import router from "@/router";

// 建议定义接口，提升开发时的代码提示
interface Workspace {
    id: string
    displayName: string
    [key: string]: any
}

export const useWorkspaceStore = defineStore('workspace', () => {
    const route = useRoute()
    const router = useRouter() // ✅ 正确：在 Store 顶层初始化

    // 1. 状态管理
    // 使用 shallowRef 提升性能（如果 Workspace 对象很复杂且你不需要深层响应）
    const currentWorkspace = ref<Workspace | null>(
        JSON.parse(localStorage.getItem('active_ws') || 'null')
    )

    // 2. 计算属性：统一暴露当前 ID
    // 核心优化：优先从 URL 获取，URL 没有再看 Store 缓存（单一事实来源原则）
    const activeId = computed(() => {
        return (route?.params?.wsId as string) || ''
    })

    // 3. 切换空间
    function switchWorkspace(ws: Workspace) {
        if (!ws) return

        // 1. 更新内部状态
        currentWorkspace.value = ws

        // 2. 持久化
        localStorage.setItem('active_ws', JSON.stringify(ws))
        localStorage.setItem('active_ws_id', ws.id)

    }

    // 4. 同步逻辑：监听路由变化（关键！）
    // 当用户点击浏览器后退或手动改 URL 时，同步更新 Store 里的对象
    // 假设你有一个所有的 workspace 列表 rows
    const rows = ref<Workspace[]>([])

    // 更新列表数据
    function setRows(data: any[]) {
        rows.value = data
    }

    watch(() => route.params.wsId, (newId) => {
        if (newId && newId !== currentWorkspace.value?.id) {
            const found = rows.value.find(item => item.id === newId)
            if (found) currentWorkspace.value = found
        }
    }, { immediate: true })

    function clear() {
        currentWorkspace.value = null
        localStorage.removeItem('active_ws')
        localStorage.removeItem('active_ws_id')
    }

    return {
        currentWorkspace,
        activeId,
        rows, // 存放列表数据
        switchWorkspace,
        setRows,
        clear
    }
})