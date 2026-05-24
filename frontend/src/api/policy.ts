import request from '@/api/request';



export const listPolicy = (data:any) => request.get('/policies/list', data);
export const createPolicy = (data:any) => request.post('/policies/create', data);
export const updatePolicy = (data:any) => request.put('/policies/create', data);
export const deletePolicy = (id:any) => request.delete(`/policies/${id}`);