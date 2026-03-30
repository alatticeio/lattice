import request from '@/api/request';

export const getDashboardOverview = () =>
    request.get('/dashboard/overview');
