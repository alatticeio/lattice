import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner' // 或者从 '@/components/ui/sonner' 引入
import { getMe, login as loginApi } from '@/api/user'
import { setToken, removeToken, hasToken } from '@/utils/auth'

export interface User {
    id: string | number
    username: string
    email: string
    role: 'admin' | 'user' | 'guest'
    avatarUrl?: string
}

export const useUserStore = defineStore('user', () => {
    const router = useRouter()

    // --- State ---
    const userInfo = ref<User | null>(null)
    const loading = ref(false)

    // --- Getters ---
    const isLoggedIn = computed(() => !!userInfo.value)

    // --- Actions ---

    /**
     * 登录逻辑
     */
    async function login(payload: any) {
        loading.value = true
        try {
            const { data, code, message } = await loginApi(payload)

            if (code === 200) {
                setToken(data.token)
                await fetchUserInfo()

                // 成功反馈
                toast.success('登录成功', {
                    description: `欢迎回来, ${userInfo.value?.username}!`,
                })

                router.push('/')
                return true
            } else {
                toast.error('登录失败', { description: message })
                return false
            }
        } catch (error: any) {
            toast.error('请求出错', {
                description: error.response?.data?.message || '服务器连接异常'
            })
            return false
        } finally {
            loading.value = false
        }
    }

    /**
     * 获取用户信息
     * 适合在 App.vue 的 onMounted 或路由守卫中调用
     */
    async function fetchUserInfo() {
        if (!hasToken()) return

        loading.value = true
        try {
            const { data, code } = await getMe()
            if (code === 200) {
                userInfo.value = data
            } else {
                // 如果后端校验失败（如 Token 伪造）
                handleAuthError()
            }
        } catch (error) {
            handleAuthError()
        } finally {
            loading.value = false
        }
    }

    /**
     * 退出登录
     */
    function logout(showToast = true) {
        removeToken()
        userInfo.value = null
        if (showToast) {
            toast.info('已退出登录')
        }
        router.push('/auth/login')
    }

    /**
     * 私有方法：处理认证失效
     */
    function handleAuthError() {
        if (hasToken()) {
            toast.error('会话已过期', { description: '请重新登录' })
        }
        logout(false)
    }

    return {
        userInfo,
        loading,
        isLoggedIn,
        login,
        logout,
        fetchUserInfo
    }
})