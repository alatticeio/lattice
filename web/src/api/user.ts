import request from '@/api/request';

// 在 @/api/user.ts 中
export interface User {
    username: string;
    email: string;
    password?: string; // 加上可选的密码字段
    namespace?:string;
    role?:string,
    remember?: boolean;
    // ... 其他字段
}

// --- 数据定义 ---
interface Binding { ns: string; role: string; quota: string; }
interface Member {
    id: number; name: string; email: string; avatar: string;
    provider: 'local' | 'dex'; role: string; sa: string;
    status: 'active' | 'pending'; lastActive: string; tenantId: string;
    vip?: string; bindings: Binding[];
}


export const registerUser = (data?: any) => request.post('/users/register', data);
export const login = (data:User) => request.post('/users/login', data);
export const add = (data?: any) => request.post("/users/add", data)
export const listUser = (data?: any) => request.get("/users/list", data)

export const deleteUser = (id:string) => request.delete(`/users/${id}`);

export const listPeer = (data?: any) => request.get('/peers/list', data);
export const updatePeer = (data?: any) => request.put('/peers/update', data);



export const getMe = (data?: any) => request.get("/users/getme", data)
export const updateMe = (data?: any) => request.put("/profile/updateProfile", data)