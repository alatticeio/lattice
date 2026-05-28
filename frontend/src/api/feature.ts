import request from '@/api/request'

export interface FeatureFlag {
  key: string
  label: string
  group: string
  enabled: boolean
}

export const getFeatures = () =>
  request.get<FeatureFlag[]>('/features')

export const updateFeature = (key: string, enabled: boolean) =>
  request.put(`/features/${key}`, { enabled })
