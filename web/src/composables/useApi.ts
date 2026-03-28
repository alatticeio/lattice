
import {inject, onMounted, reactive, ref, watch} from 'vue'
import type {ApiResponse, PageParams, TableOptions} from '@/types/table';

/**
 * 通用请求封装钩子
 * @param {Function} apiFunc - 你的 API 请求函数 (来自 @/api/*)
 */
export function useApi(apiFunc?: any) {
    const data = ref(null)
    const loading = ref(false)
    const error = ref(null)

    // 修复1：给 rest 参数加上类型注解
    const execute = async (...args: any[]) => {
        loading.value = true
        error.value = null

        try {
            const res = await apiFunc(...args)
            data.value = res.data || res
            return {success: true, data: data.value}
        } catch (err) {
            // 修复2：将 err 断言为 any 再访问属性
            const e = err as any
            const errorMsg = e?.response?.data?.message || e?.message || '请求失败'
            error.value = errorMsg
            return {success: false, error: errorMsg}
        } finally {
            loading.value = false
        }
    }

    return {
        data,
        loading,
        error,
        execute
    }
}


// T 代表行数据的类型，例如 User 或 Workspace
export function useTable<T>(
    apiFunc: (p: PageParams) => Promise<ApiResponse<T>>,
    options: TableOptions = {}
) {
    const {
        initialParams = {},
        immediate = true,
        successMsg = '',
        errorMsg = '获取数据失败'
    } = options;

    const toast = inject<(msg: string, type: 'success' | 'error') => void>('globalToast');

    const loading = ref(false);
    const rows = ref<T[]>([]) as any;
    const total = ref(0);

    const params = reactive<PageParams>({
        page: 1,
        pageSize: 4,
        search: '',
        ...initialParams
    });

    const refresh = async () => {
        loading.value = true;
        try {
            const {code, data, msg} = await apiFunc(params);

            if (code === 200) {
                rows.value = data?.list || [];
                total.value = data?.total || 0;
                if (successMsg) toast?.(successMsg, 'success');
            } else {
                toast?.(msg || errorMsg, 'error');
            }
        } catch (err) {
            console.error('Table fetch error:', err);
            toast?.('网络请求异常', 'error');
        } finally {
            loading.value = false;
        }
    };

    watch(() => params.page, refresh);

    watch(() => params.search, () => {
        params.value.page = 1;
        refresh();
    });

    if (immediate) onMounted(refresh);

    return {loading, rows, total, params, refresh};
}

// 修复3：给 useAction 的参数加上类型注解
export function useAction(apiFunc: (...args: any[]) => Promise<any>, options: Record<string, any> = {}) {
    const toast = inject<(msg: string, type: 'success' | 'error') => void>('globalToast')
    const loading = ref(false)

    const execute = async (params: any) => {
        loading.value = true
        try {
            const {code, data, msg} = await apiFunc(params)
            if (code === 200) {
                if (!options.silent && options.successMsg) {
                    toast?.(options.successMsg, 'success')
                }
                options.onSuccess?.(data)
                return data
            } else {
                if (!options.silentError) {
                    toast?.(msg || options.errorMsg || '请求失败', 'error')
                }
            }
        } catch (err) {
            // 修复4：第二个参数应为 'error' 而不是 err
            toast?.('系统繁忙，请稍后再试', 'error')
            return false
        } finally {
            loading.value = false
        }
    }

    return {loading, execute}
}