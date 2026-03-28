import { defineStore } from 'pinia'
import {reactive, ref} from 'vue'
import { getMe } from '@/api/user'

// 1. 定义用户信息接口
export interface User {
    id: string | number
    username: string
    email: string
    role: 'admin' | 'user' | 'guest'
    avatarUrl?: string
    // 根据你的 API 返回结构继续添加字段
}

export const useUserStore = defineStore('user', () => {
    // =========================================================
    // 1. DATA (State)
    // =========================================================

    // 使用泛型约束，初始值为 null
    const userInfo = ref<User | null>(null)
    const loading = ref(false)

    // =========================================================
    // 2. ACTIONS (Methods)
    // =========================================================
    const actions = {
        async fetchUserInfo() {
            loading.value = true
            try {
                const { data, code } = await getMe()
                if (code === 200) {
                    userInfo.value = data
                }
            } catch (error) {
                userInfo.value = null
                console.error('用户信息加载失败:', error)
            } finally {
                loading.value = false
            }
        },

        // 清除状态（注销时使用）
        clearUser() {
            userInfo.value = null
        }
    }

    return {
        userInfo,
        loading,
        actions
    }
})