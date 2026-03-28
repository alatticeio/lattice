import { ref, watchEffect } from 'vue'

export function useTheme() {
    const theme = ref(localStorage.getItem('theme') || 'light')

    watchEffect(() => {
        document.documentElement.setAttribute('data-theme', theme.value)
        localStorage.setItem('theme', theme.value)
    })

    function toggleTheme() {
        theme.value = theme.value === 'light' ? 'dark' : 'light'
    }

    return { theme, toggleTheme }
}

/**
 * 根据名称获取固定的 Tailwind 颜色类
 * @param {string} name
 * @returns {string} Tailwind color class
 */
export const getAvatarColor = (name:string) => {
    const colors = [
        'bg-indigo-500',
        'bg-emerald-500',
        'bg-rose-500',
        'bg-amber-500',
        'bg-sky-500',
        'bg-violet-500',
        'bg-orange-500',
        'bg-fuchsia-500',
        'bg-teal-500'
    ];

    if (!name) return colors[0];

    // 取第一个字符的 Unicode 码点
    const charCode = name.charCodeAt(0);

    // 对颜色池长度取模
    return colors[charCode % colors.length];
};

/**
 * 获取展示用的首字母（处理中英文）
 */
export const getFirstChar = (name:string) => {
    return name ? name.charAt(0).toUpperCase() : '?';
};