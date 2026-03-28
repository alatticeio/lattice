import { defineStore } from 'pinia'
import { ref, watchEffect } from 'vue'

export const useThemeStore = defineStore('theme', () => {
    // =========================================================
    // 1. DATA (State)
    // =========================================================

    // 从本地存储初始化，如果没有则默认为 'light'
    const theme = ref(localStorage.getItem('theme') || 'light')

    // 副作用处理：监听变化并同步到 DOM 和 Storage
    watchEffect(() => {
        document.documentElement.setAttribute('data-theme', theme.value)
        localStorage.setItem('theme', theme.value)
    })

    // =========================================================
    // 2. ACTIONS (Methods)
    // =========================================================
    const actions = {
        toggleTheme() {
            theme.value = theme.value === 'light' ? 'dark' : 'light'
        },

        setTheme(newTheme: 'light' | 'dark') {
            theme.value = newTheme
        },

        /**
         * 根据名称获取固定的 DaisyUI/Tailwind 颜色类
         */
        getAvatarColor(name: string) {
            const colors = [
                'bg-primary',   // 建议优先使用主题色
                'bg-secondary',
                'bg-accent',
                'bg-neutral',
                'bg-info',
                'bg-success',
                'bg-warning',
                'bg-error',
                'bg-indigo-500',
                'bg-emerald-500'
            ]
            if (!name) return colors[0]
            const charCode = name.charCodeAt(0)
            return colors[charCode % colors.length]
        },

        /**
         * 获取展示用的首字母
         */
        getFirstChar(name: string) {
            return name ? name.charAt(0).toUpperCase() : '?'
        }
    }

    return {
        theme,
        actions
    }
})