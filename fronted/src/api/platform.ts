import request from '@/api/request'

export interface PlatformSettings {
  nats_url: string
}

export const getPlatformSettings = () =>
  request.get<PlatformSettings>('/settings/platform')

export const updatePlatformSettings = (data: PlatformSettings) =>
  request.put('/settings/platform', data)
