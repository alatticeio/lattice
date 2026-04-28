import request from '@/api/request';

export interface User {
  username: string
  email: string
  password?: string
  namespace?: string
  role?: string
  remember?: boolean
}

export interface UserVo {
  id: string
  name: string
  email: string
  avatar?: string
  role: 'platform_admin' | 'user' | ''
  source?: string       // "local" | "invitation" | "github" | "dex" | ...
  inviterName?: string  // set when source === "invitation"
  registeredAt?: string // ISO-8601
}

export const registerUser = (data?: any) => request.post('/users/register', data)
export const login = (data: User) => request.post('/users/login', data)
export const add = (data?: any) => request.post('/users/add', data)
export const listUser = (params?: any) => request.get('/users/list', params)
export const deleteUser = (id: string) => request.delete(`/users/${id}`)
export const updateSystemRole = (id: string, systemRole: string) =>
  request.patch(`/users/${id}/system-role`, { systemRole })

export const listPeer = (data?: any) => request.get('/peers/list', data)
export const updatePeer = (data?: any) => request.put('/peers/update', data)
export const disablePeer = (name: string) => request.put(`/peers/${name}/disable`, {})
export const enablePeer = (name: string) => request.put(`/peers/${name}/enable`, {})
export const deletePeer = (name: string) => request.delete(`/peers/${name}`)

export const listPeerings = () => request.get('/peering/list')
export const createPeering = (data: { name?: string; namespaceB: string; networkB?: string; peeringMode?: string }) =>
  request.post('/peering', data)
export const deletePeering = (name: string) => request.delete(`/peering/${name}`)

export const getMe = (data?: any) => request.get('/users/getme', data)
export const updateMe = (data?: any) => request.put('/profile/updateProfile', data)
export const uploadAvatar = (formData: FormData) =>
  request.post<{ data: { url: string } }>('/profile/avatar', formData)