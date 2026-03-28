import axios, {
    InternalAxiosRequestConfig,
    AxiosResponse,
    AxiosInstance,
    AxiosRequestConfig
} from 'axios';
import router from '@/router';
import NProgress from '@/types/progress';
import type { ApiResponse } from '@/types/api';

// 1. 创建实例
const service: AxiosInstance = axios.create({
    baseURL: (import.meta.env.VITE_API_BASE as string) || '/api/v1',
    timeout: 30000,
    headers: { 'Content-Type': 'application/json' }
});

// 2. 请求拦截器
service.interceptors.request.use(
    (config: InternalAxiosRequestConfig) => {
        NProgress.start();

        // 处理 Token
        const token = localStorage.getItem('wf_token');
        if (token && config.headers) {
            config.headers['Authorization'] = `Bearer ${token}`;
        }

        // 安全获取 workspaceId
        const params = router.currentRoute?.value?.params;
        const workspaceId = params?.wsId as string | undefined;

        if (workspaceId && workspaceId !== 'all' && config.headers) {
            config.headers['X-Workspace-Id'] = workspaceId;
        }

        return config;
    },
    (error) => {
        NProgress.done();
        return Promise.reject(error);
    }
);

// 3. 响应拦截器
service.interceptors.response.use(
    (response: AxiosResponse<ApiResponse>) => {
        NProgress.done();
        // 直接返回后端定义的 data 结构
        return response.data as any;
    },
    (error) => {
        NProgress.done();
        const { response } = error;
        let message = '网络异常，请稍后再试';

        if (response) {
            switch (response.status) {
                case 401:
                    message = '登录已过期，请重新登录';
                    break;
                case 403:
                    message = '权限不足，无法操作';
                    break;
                default:
                    message = response.data?.message || message;
            }
        }
        console.error('API Error:', message);
        return Promise.reject(error);
    }
);

// 4. 封装常用的请求方法 (支持泛型 T)
const request = {
    get<T = any>(url: string, params: object = {}, config: AxiosRequestConfig = {}): Promise<T> {
        return service.get(url, { params, ...config });
    },
    post<T = any>(url: string, data: object = {}, config: AxiosRequestConfig = {}): Promise<T> {
        return service.post(url, data, config);
    },
    put<T = any>(url: string, data: object = {}, config: AxiosRequestConfig = {}): Promise<T> {
        return service.put(url, data, config);
    },
    delete<T = any>(url: string, config: AxiosRequestConfig = {}): Promise<T> {
        return service.delete(url, config);
    },
    upload<T = any>(url: string, file: File): Promise<T> {
        const formData = new FormData();
        formData.append('file', file);
        return service.post(url, formData, {
            headers: { 'Content-Type': 'multipart/form-data' }
        });
    }
};

export default request;