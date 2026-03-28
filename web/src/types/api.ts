// types/api.ts
export interface ApiResponse<T = any> {
    code: number;
    data: T;
    message?: string;
    events?: any[]; // 针对你 Wireflow 项目的特殊字段
}