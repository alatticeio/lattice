import request from '@/api/request'
import type {
  AlertRule,
  AlertHistory,
  AlertChannel,
  AlertSilence,
  CreateAlertRuleRequest,
  CreateChannelRequest,
  CreateSilenceRequest,
} from '@/types/alert'

export const listAlertRules = (wsID: string) =>
  request.get<AlertRule[]>(`/alerts/rules?workspace_id=${wsID}`)

export const getAlertRule = (id: string) =>
  request.get<AlertRule>(`/alerts/rules/${id}`)

export const createAlertRule = (wsID: string, data: CreateAlertRuleRequest) =>
  request.post<AlertRule>(`/alerts/rules?workspace_id=${wsID}`, data)

export const updateAlertRule = (id: string, data: CreateAlertRuleRequest) =>
  request.put<AlertRule>(`/alerts/rules/${id}`, data)

export const deleteAlertRule = (id: string) =>
  request.delete(`/alerts/rules/${id}`)

export const listAlertHistory = (wsID: string, page = 1, pageSize = 20) =>
  request.get<{ items: AlertHistory[]; total: number }>(
    `/alerts/history?workspace_id=${wsID}&page=${page}&pageSize=${pageSize}`
  )

export const listAlertChannels = (wsID: string) =>
  request.get<AlertChannel[]>(`/alerts/channels?workspace_id=${wsID}`)

export const createAlertChannel = (wsID: string, data: CreateChannelRequest) =>
  request.post<AlertChannel>(`/alerts/channels?workspace_id=${wsID}`, data)

export const updateAlertChannel = (id: string, data: CreateChannelRequest) =>
  request.put<AlertChannel>(`/alerts/channels/${id}`, data)

export const deleteAlertChannel = (id: string) =>
  request.delete(`/alerts/channels/${id}`)

export const listAlertSilences = (wsID: string) =>
  request.get<AlertSilence[]>(`/alerts/silences?workspace_id=${wsID}`)

export const createAlertSilence = (wsID: string, data: CreateSilenceRequest) =>
  request.post<AlertSilence>(`/alerts/silences?workspace_id=${wsID}`, data)

export const deleteAlertSilence = (id: string) =>
  request.delete(`/alerts/silences/${id}`)
