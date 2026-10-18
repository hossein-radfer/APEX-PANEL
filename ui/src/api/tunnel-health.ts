import {
  GraphResponseSchema,
  RedlineEntriesResponseSchema,
  RedlineEntry,
  TunnelActionLog,
  TunnelActionLogsResponseSchema,
  TunnelAiSettings,
  TunnelAiSettingsResponseSchema,
  TunnelBackupProbeResult,
  TunnelBackupProbeResultsResponseSchema,
  TunnelGraph,
  TunnelHealthEvent,
  TunnelHealthEventsResponseSchema,
  TunnelHealthScorePoint,
  TunnelHealthScoresResponseSchema,
  TunnelHealthStatus,
  TunnelHealthStatusesResponseSchema,
  TunnelIncidentDiagnosis,
  TunnelIncidentDiagnosesResponseSchema,
  TunnelMap,
  TunnelMapResponseSchema,
  TunnelPoliciesResponseSchema,
  TunnelPolicy,
  TunnelPolicyResponseSchema,
  UserManagerProtocolHealthEvent,
  UserManagerProtocolHealthEventsResponseSchema,
  UserManagerProtocolHealthStatus,
  UserManagerProtocolHealthStatusesResponseSchema,
} from '@/schema/tunnel-health.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchTunnelHealthStatuses = async (
  serverId?: number
): Promise<TunnelHealthStatus[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/statuses', {
    params: { server_id: serverId },
  })
  const parsed = TunnelHealthStatusesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchTunnelHealthEvents = async (
  limit = 100,
  serverId?: number
): Promise<TunnelHealthEvent[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/events', {
    params: { limit, server_id: serverId },
  })
  const parsed = TunnelHealthEventsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchTunnelActionLogs = async (
  limit = 100,
  serverId?: number
): Promise<TunnelActionLog[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/actions', {
    params: { limit, server_id: serverId },
  })
  const parsed = TunnelActionLogsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchTunnelAiSettings = async (): Promise<TunnelAiSettings> => {
  const { data } = await axiosInstance.get('/tunnel-health/settings')
  const parsed = TunnelAiSettingsResponseSchema.parse(data)
  return parsed.data
}

export const updateTunnelAiSettings = async (
  req: Partial<TunnelAiSettings>
): Promise<TunnelAiSettings> => {
  const { data } = await axiosInstance.patch('/tunnel-health/settings', {
    dry_run: req.dry_run,
    emergency_stop: req.emergency_stop,
  })
  const parsed = TunnelAiSettingsResponseSchema.parse(data)
  return parsed.data
}

export const fetchTunnelPolicies = async (
  serverId?: number
): Promise<TunnelPolicy[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/policies', {
    params: { server_id: serverId },
  })
  const parsed = TunnelPoliciesResponseSchema.parse(data)
  return parsed.data || []
}

export const updateTunnelPolicy = async (
  policy: TunnelPolicy
): Promise<TunnelPolicy> => {
  const { data } = await axiosInstance.put(
    `/tunnel-health/policies/${policy.id}`,
    {
      detection: policy.detection,
      level1: policy.level1,
      level2: policy.level2,
      level3: policy.level3,
      fallback: policy.fallback,
      anti_flapping: policy.anti_flapping,
    }
  )
  const parsed = TunnelPolicyResponseSchema.parse(data)
  return parsed.data
}

export const fetchRedlineEntries = async (): Promise<RedlineEntry[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/redline')
  const parsed = RedlineEntriesResponseSchema.parse(data)
  return parsed.data || []
}

export const addRedlineEntry = async (req: {
  kind: 'interface' | 'ip' | 'port'
  value: string
  comment?: string
}): Promise<RedlineEntry> => {
  const { data } = await axiosInstance.post('/tunnel-health/redline', req)
  return data.data
}

export const removeRedlineEntry = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/tunnel-health/redline/${id}`)
}

export const fetchLatestGraph = async (
  serverId: number
): Promise<TunnelGraph | null> => {
  const { data } = await axiosInstance.get('/tunnel-health/graph', {
    params: { server_id: serverId },
  })
  const parsed = GraphResponseSchema.parse(data)
  return parsed.data
}

export const fetchTunnelMap = async (
  serverId: number
): Promise<TunnelMap | null> => {
  const { data } = await axiosInstance.get('/tunnel-health/tunnel-map', {
    params: { server_id: serverId },
  })
  const parsed = TunnelMapResponseSchema.parse(data)
  return parsed.data
}

export const fetchTunnelHealthScoreHistory = async (
  serverId: number,
  interfaceName: string,
  limit = 20
): Promise<TunnelHealthScorePoint[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/health-scores', {
    params: { server_id: serverId, interface_name: interfaceName, limit },
  })
  const parsed = TunnelHealthScoresResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchTunnelIncidentDiagnoses = async (
  limit = 100,
  serverId?: number
): Promise<TunnelIncidentDiagnosis[]> => {
  const { data } = await axiosInstance.get(
    '/tunnel-health/incident-diagnoses',
    { params: { limit, server_id: serverId } }
  )
  const parsed = TunnelIncidentDiagnosesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchTunnelBackupProbeResults = async (
  limit = 100,
  serverId?: number
): Promise<TunnelBackupProbeResult[]> => {
  const { data } = await axiosInstance.get('/tunnel-health/backup-probes', {
    params: { limit, server_id: serverId },
  })
  const parsed = TunnelBackupProbeResultsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserManagerProtocolStatuses = async (): Promise<
  UserManagerProtocolHealthStatus[]
> => {
  const { data } = await axiosInstance.get(
    '/tunnel-health/user-manager-protocols'
  )
  const parsed = UserManagerProtocolHealthStatusesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserManagerProtocolEvents = async (
  limit = 100
): Promise<UserManagerProtocolHealthEvent[]> => {
  const { data } = await axiosInstance.get(
    '/tunnel-health/user-manager-protocols/events',
    { params: { limit } }
  )
  const parsed = UserManagerProtocolHealthEventsResponseSchema.parse(data)
  return parsed.data || []
}
