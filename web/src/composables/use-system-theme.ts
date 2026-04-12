import { ref, watchEffect, onMounted } from 'vue'

type Theme = 'light' | 'dark' | 'system'

export function useSystemTheme() {
    // 从本地存储读取初始化状态，默认 'system'
    const theme = ref<Theme>((localStorage.getItem('app-theme') as Theme) || 'system')

    const applyTheme = (targetTheme: Theme) => {
        const root = window.document.documentElement
        // 移除旧的类名
        root.classList.remove('light', 'dark')

        let effectiveTheme = targetTheme

        // 如果是系统模式，检测媒体查询
        if (targetTheme === 'system') {
            effectiveTheme = window.matchMedia('(prefers-color-scheme: dark)').matches
                ? 'dark'
                : 'light'
        }

        root.classList.add(effectiveTheme)

        // Radix UI 或其他库有时会用到 color-scheme 属性
        root.style.colorScheme = effectiveTheme
    }

    // 监听主题变化并持久化
    watchEffect(() => {
        localStorage.setItem('app-theme', theme.value)
        applyTheme(theme.value)
    })

    onMounted(() => {
        // 监听系统主题实时变化（比如黄昏自动切换）
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
        const handler = () => {
            if (theme.value === 'system') applyTheme('system')
        }

        mediaQuery.addEventListener('change', handler)
    })

    return {
        theme,
        setTheme: (newTheme: Theme) => { theme.value = newTheme }
    }
}