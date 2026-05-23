import { ref, reactive } from 'vue'
import { toast } from 'vue-sonner'

interface UseTableOptions {
  successMsg?: string
  errorMsg?: string
  immediate?: boolean
  initialParams?: Record<string, any>
}

export function useTable(apiFn: (params?: any) => Promise<any>, options: UseTableOptions = {}) {
  const rows = ref<any[]>([])
  const total = ref(0)
  const loading = ref(false)
  const params = reactive<{ page: number; pageSize: number; search?: string; [key: string]: any }>({ page: 1, pageSize: 20, ...options.initialParams })

  async function refresh(extraParams?: Record<string, any>) {
    loading.value = true
    try {
      const res = await apiFn({ ...params, ...extraParams })
      const data = res?.data
      rows.value = data?.list ?? data?.rows ?? data?.items ?? (Array.isArray(data) ? data : [])
      total.value = data?.total ?? rows.value.length
      if (options.successMsg) toast.success(options.successMsg)
    } catch (e: any) {
      toast.error(e?.response?.data?.message ?? '加载失败')
    } finally {
      loading.value = false
    }
  }

  return { rows, total, loading, params, refresh }
}

interface UseActionOptions {
  successMsg?: string
  errorMsg?: string
  onSuccess?: (res?: any) => void
  onError?: (e?: any) => void
}

export function useAction(apiFn: (data?: any) => Promise<any>, options: UseActionOptions = {}) {
  const loading = ref(false)

  async function execute(data?: any) {
    loading.value = true
    try {
      const res = await apiFn(data)
      if (options.successMsg) toast.success(options.successMsg)
      options.onSuccess?.(res)
      return res
    } catch (e: any) {
      const msg = options.errorMsg ?? e?.response?.data?.message ?? '操作失败'
      toast.error(msg)
      options.onError?.(e)
      throw e
    } finally {
      loading.value = false
    }
  }

  return { execute, loading }
}
