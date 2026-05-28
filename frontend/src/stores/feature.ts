import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getFeatures, type FeatureFlag } from '@/api/feature'

export const useFeatureStore = defineStore('feature', () => {
  const flags = ref<Record<string, boolean>>({})
  const loaded = ref(false)
  const allFlags = ref<FeatureFlag[]>([])

  async function fetchFeatures() {
    try {
      const res = await getFeatures()
      const list: FeatureFlag[] = Array.isArray(res) ? res : (res as any).data ?? []
      allFlags.value = list
      const map: Record<string, boolean> = {}
      for (const f of list) {
        map[f.key] = f.enabled
      }
      flags.value = map
      loaded.value = true
    } catch {
      // On failure, default all to enabled
      loaded.value = true
    }
  }

  function isEnabled(key: string): boolean {
    if (!(key in flags.value)) return true
    return flags.value[key]
  }

  function setFlag(key: string, enabled: boolean) {
    flags.value[key] = enabled
    const item = allFlags.value.find(f => f.key === key)
    if (item) item.enabled = enabled
  }

  return { flags, loaded, allFlags, fetchFeatures, isEnabled, setFlag }
})
