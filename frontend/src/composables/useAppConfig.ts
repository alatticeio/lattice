import { ref, watch } from 'vue'

export type Theme = 'light' | 'dark' | 'system'
export type ColorScheme = 'zinc' | 'blue' | 'violet' | 'green' | 'orange' | 'rose'
export type RadiusValue = '0' | '0.25' | '0.5' | '0.625' | '0.75' | '1'
export type FontFamily = 'system' | 'mono' | 'serif'

export interface AppConfig {
  theme: Theme
  color: ColorScheme
  radius: RadiusValue
  font: FontFamily
}

interface ColorVars {
  primary: string
  primaryFg: string
  /** sidebar active / hover background */
  sidebarAccent: string
  sidebarAccentFg: string
  /** active indicator dot / icon in sidebar */
  sidebarPrimary: string
  sidebarPrimaryFg: string
}

export const COLOR_PRESETS: Record<ColorScheme, {
  swatch: string
  light: ColorVars
  dark: ColorVars
}> = {
  zinc: {
    swatch: 'oklch(0.21 0.006 285.885)',
    light: {
      primary:          'oklch(0.21 0.006 285.885)',
      primaryFg:        'oklch(0.985 0 0)',
      sidebarAccent:    'oklch(0.967 0.001 286.375)',
      sidebarAccentFg:  'oklch(0.21 0.006 285.885)',
      sidebarPrimary:   'oklch(0.21 0.006 285.885)',
      sidebarPrimaryFg: 'oklch(0.985 0 0)',
    },
    dark: {
      primary:          'oklch(0.92 0.004 286.32)',
      primaryFg:        'oklch(0.21 0.006 285.885)',
      sidebarAccent:    'oklch(0.274 0.006 286.033)',
      sidebarAccentFg:  'oklch(0.985 0 0)',
      sidebarPrimary:   'oklch(0.488 0.243 264.376)',
      sidebarPrimaryFg: 'oklch(0.985 0 0)',
    },
  },
  blue: {
    swatch: 'oklch(0.546 0.245 262.881)',
    light: {
      primary:          'oklch(0.546 0.245 262.881)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.93 0.04 262)',
      sidebarAccentFg:  'oklch(0.38 0.18 262)',
      sidebarPrimary:   'oklch(0.546 0.245 262.881)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
    dark: {
      primary:          'oklch(0.623 0.214 259.815)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.29 0.07 262)',
      sidebarAccentFg:  'oklch(0.82 0.1 262)',
      sidebarPrimary:   'oklch(0.623 0.214 259.815)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
  },
  violet: {
    swatch: 'oklch(0.541 0.281 293.009)',
    light: {
      primary:          'oklch(0.541 0.281 293.009)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.93 0.04 293)',
      sidebarAccentFg:  'oklch(0.36 0.18 293)',
      sidebarPrimary:   'oklch(0.541 0.281 293.009)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
    dark: {
      primary:          'oklch(0.702 0.225 296.787)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.29 0.07 293)',
      sidebarAccentFg:  'oklch(0.82 0.1 293)',
      sidebarPrimary:   'oklch(0.702 0.225 296.787)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
  },
  green: {
    swatch: 'oklch(0.527 0.154 150.069)',
    light: {
      primary:          'oklch(0.527 0.154 150.069)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.93 0.05 150)',
      sidebarAccentFg:  'oklch(0.36 0.12 150)',
      sidebarPrimary:   'oklch(0.527 0.154 150.069)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
    dark: {
      primary:          'oklch(0.696 0.17 162.48)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.27 0.06 150)',
      sidebarAccentFg:  'oklch(0.82 0.1 150)',
      sidebarPrimary:   'oklch(0.696 0.17 162.48)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
  },
  orange: {
    swatch: 'oklch(0.646 0.222 41.116)',
    light: {
      primary:          'oklch(0.646 0.222 41.116)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.94 0.06 55)',
      sidebarAccentFg:  'oklch(0.42 0.14 41)',
      sidebarPrimary:   'oklch(0.646 0.222 41.116)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
    dark: {
      primary:          'oklch(0.769 0.188 70.08)',
      primaryFg:        'oklch(0.141 0 0)',
      sidebarAccent:    'oklch(0.29 0.07 55)',
      sidebarAccentFg:  'oklch(0.85 0.12 55)',
      sidebarPrimary:   'oklch(0.769 0.188 70.08)',
      sidebarPrimaryFg: 'oklch(0.141 0 0)',
    },
  },
  rose: {
    swatch: 'oklch(0.577 0.245 27.325)',
    light: {
      primary:          'oklch(0.577 0.245 27.325)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.94 0.05 15)',
      sidebarAccentFg:  'oklch(0.4 0.16 15)',
      sidebarPrimary:   'oklch(0.577 0.245 27.325)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
    dark: {
      primary:          'oklch(0.704 0.191 22.216)',
      primaryFg:        'oklch(1 0 0)',
      sidebarAccent:    'oklch(0.29 0.07 15)',
      sidebarAccentFg:  'oklch(0.85 0.1 15)',
      sidebarPrimary:   'oklch(0.704 0.191 22.216)',
      sidebarPrimaryFg: 'oklch(1 0 0)',
    },
  },
}

export const RADIUS_OPTIONS: { label: string; value: RadiusValue }[] = [
  { label: 'None',    value: '0' },
  { label: 'Small',   value: '0.25' },
  { label: 'Default', value: '0.625' },
  { label: 'Large',   value: '0.75' },
  { label: 'Full',    value: '1' },
]

export const FONT_OPTIONS: { label: string; value: FontFamily; style: string }[] = [
  { label: 'System', value: 'system', style: 'system-ui, sans-serif' },
  { label: 'Mono',   value: 'mono',   style: '"JetBrains Mono", monospace' },
  { label: 'Serif',  value: 'serif',  style: 'Georgia, serif' },
]

const CONFIG_KEY = '__APP_SETTINGS_V1__'

function defaults(): AppConfig {
  return { theme: 'system', color: 'zinc', radius: '0.625', font: 'system' }
}

function load(): AppConfig {
  try {
    const s = localStorage.getItem(CONFIG_KEY)
    if (s) return { ...defaults(), ...JSON.parse(s) }
  } catch { /* ignore */ }
  return defaults()
}

function resolveIsDark(theme: Theme): boolean {
  if (theme === 'dark') return true
  if (theme === 'light') return false
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyConfig(c: AppConfig) {
  const isDark = resolveIsDark(c.theme)

  // 1. Theme class
  document.documentElement.classList.toggle('dark', isDark)

  // 2. Inject/update a <style> that overrides both :root and .dark for
  //    primary + all sidebar color variables
  const p = COLOR_PRESETS[c.color]
  const l = p.light
  const d = p.dark

  let el = document.getElementById('app-color-vars') as HTMLStyleElement | null
  if (!el) {
    el = document.createElement('style')
    el.id = 'app-color-vars'
    document.head.appendChild(el)
  }

  el.textContent = `
    :root {
      --primary:                  ${l.primary};
      --primary-foreground:       ${l.primaryFg};
      --sidebar-accent:           ${l.sidebarAccent};
      --sidebar-accent-foreground:${l.sidebarAccentFg};
      --sidebar-primary:          ${l.sidebarPrimary};
      --sidebar-primary-foreground:${l.sidebarPrimaryFg};
    }
    .dark {
      --primary:                  ${d.primary};
      --primary-foreground:       ${d.primaryFg};
      --sidebar-accent:           ${d.sidebarAccent};
      --sidebar-accent-foreground:${d.sidebarAccentFg};
      --sidebar-primary:          ${d.sidebarPrimary};
      --sidebar-primary-foreground:${d.sidebarPrimaryFg};
    }
  `

  // 3. Radius
  document.documentElement.style.setProperty('--radius', `${c.radius}rem`)

  // 4. Font
  const font = FONT_OPTIONS.find(f => f.value === c.font)
  document.documentElement.style.fontFamily = font ? font.style : ''
}

// ── Singleton ──────────────────────────────────────────────────────────────
const config = ref<AppConfig>(load())

applyConfig(config.value)

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (config.value.theme === 'system') applyConfig(config.value)
})

export function useAppConfig() {
  watch(config, (c) => {
    applyConfig(c)
    localStorage.setItem(CONFIG_KEY, JSON.stringify(c))
  }, { deep: true })

  function reset() {
    config.value = defaults()
  }

  return { config, reset }
}
