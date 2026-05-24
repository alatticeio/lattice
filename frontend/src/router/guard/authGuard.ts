import { hasToken, removeToken } from '@/utils/auth'
import { useUserStore } from '@/stores/user'
import type { Router } from "vue-router";

export function setupAuthGuard(router: Router) {
    // 1. 定义免登录白名单
    const whiteList = ['/', '/auth/login', '/auth/signup']
    const whiteListPrefixes = ['/invite/']

    router.beforeEach(async (to, _from, next) => {
        const userStore = useUserStore()
        const tokenExists = hasToken()

        // 判断当前页面是否在白名单中
        const isWhiteListed = whiteList.includes(to.path) ||
            whiteListPrefixes.some(p => to.path.startsWith(p))

        // 2. 尝试找回身份（仅当有 Token 且内存没数据时）
        if (tokenExists && !userStore.isLoggedIn) {
            try {
                await userStore.fetchUserInfo()
            } catch (error) {
                console.error('身份验证失败，清理凭证')
                removeToken()
            }
        }

        const loggedIn = userStore.isLoggedIn

        // 3. 拦截逻辑判断
        if (loggedIn) {
            // --- 已登录状态 ---
            if (to.path === '/auth/login' || to.path === '/auth/signup') {
                // 已登录用户不允许再去登录/注册页，重定向到首页
                next('/')
            } else {
                // 其他页面（包括首页和受保护页面）一律放行
                next()
            }
        } else {
            // --- 未登录状态 ---
            if (isWhiteListed) {
                // 在白名单内的路径（首页、登录、注册），直接放行
                next()
            } else {
                // 不在白名单且未登录，强制跳转登录，并记录重定向地址
                next(`/auth/login?redirect=${to.fullPath}`)
            }
        }
    })
}