import { ref, watchEffect } from 'vue'

function getInitial(): boolean {
  const saved = localStorage.getItem('theme')
  if (saved) return saved === 'dark'
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

const isDark = ref(getInitial())

// Apply immediately on load
if (isDark.value) document.documentElement.classList.add('dark')

export function useTheme() {
  function toggle() {
    isDark.value = !isDark.value
  }

  watchEffect(() => {
    document.documentElement.classList.toggle('dark', isDark.value)
    localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
  })

  return { isDark, toggle }
}
