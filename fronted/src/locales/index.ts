import { createI18n } from 'vue-i18n'

// zh-CN
import zhCommon  from './zh-CN/common.json'
import zhManage  from './zh-CN/manage.json'
import zhSettings from './zh-CN/settings.json'

// en
import enCommon  from './en/common.json'
import enManage  from './en/manage.json'
import enSettings from './en/settings.json'

export type Locale = 'zh-CN' | 'en'

const STORAGE_KEY = 'wireflow_lang'

function detectLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY) as Locale | null
  if (saved === 'zh-CN' || saved === 'en') return saved
  return navigator.language.startsWith('zh') ? 'zh-CN' : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': {
      common: zhCommon,
      manage: zhManage,
      settings: zhSettings,
    },
    en: {
      common: enCommon,
      manage: enManage,
      settings: enSettings,
    },
  },
})

export function setLocale(lang: Locale) {
  ;(i18n.global.locale as any).value = lang
  localStorage.setItem(STORAGE_KEY, lang)
  document.documentElement.lang = lang
}

export function currentLocale(): Locale {
  return (i18n.global.locale as any).value as Locale
}
