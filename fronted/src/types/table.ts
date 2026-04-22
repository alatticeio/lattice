// 分页请求参数
export interface PageParams {
    page: number;
    pageSize: number;
    search?: string;
    [key: string]: any; // 允许扩展其他筛选字段
}

// 后端统一响应结构 (对应你 Go 的 API 返回)
export interface ApiResponse<T> {
    code: number;
    data: {
        list: T[];
        total: number;
    };
    msg: string;
}

// useTable 的配置项
export interface TableOptions {
    initialParams?: Partial<PageParams>;
    immediate?: boolean;
    successMsg?: string;
    errorMsg?: string;
}