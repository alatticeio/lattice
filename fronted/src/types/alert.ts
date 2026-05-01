export interface AlertRule {
  id: string
  name: string
  workspace_id: string
  enabled: boolean
  metric_type: string
  operator: string
  threshold: number
  duration: string
  lookback: string
  group_by: string
  for_each: boolean
  channels: string
  severity: 'critical' | 'warning' | 'info'
  message: string
  silence_until?: string
  created_at: string
  updated_at: string
}

export interface AlertHistory {
  id: string
  rule_id: string
  workspace_id: string
  status: 'firing' | 'resolved'
  severity: string
  labels: string
  value: number
  message: string
  started_at: string
  ended_at?: string
  notified: boolean
}

export interface AlertChannel {
  id: string
  name: string
  workspace_id: string
  type: 'email' | 'webhook' | 'dingtalk' | 'slack'
  config: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface AlertSilence {
  id: string
  workspace_id: string
  created_by: string
  matchers: string
  comment: string
  starts_at: string
  ends_at: string
  created_at: string
}

export interface CreateAlertRuleRequest {
  name: string
  metric_type: string
  operator: string
  threshold: number
  duration: string
  lookback: string
  group_by?: string[]
  for_each?: boolean
  channels?: string[]
  severity: string
  message: string
}

export interface CreateChannelRequest {
  name: string
  type: string
  config: Record<string, any>
}

export interface CreateSilenceRequest {
  matchers: Record<string, string>[]
  comment: string
  starts_at: string
  ends_at: string
}
