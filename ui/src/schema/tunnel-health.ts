import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const TunnelHealthSeverityEnum = z.enum([
  'healthy',
  'suspect',
  'confirmed_down',
])

export const TunnelHealthStatusSchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  interface_type: z.string().optional(),
  severity: TunnelHealthSeverityEnum,
  interface_running: z.boolean(),
  interface_disabled: z.boolean(),
  worst_peer_last_handshake_age_seconds: z.number().nullable().optional(),
  tx_rx_asymmetry_detected: z.boolean(),
  last_polled_at: z.string(),
})
export const TunnelHealthStatusesResponseSchema = createApiResponseSchema(
  z.array(TunnelHealthStatusSchema)
)
export type TunnelHealthStatus = z.infer<typeof TunnelHealthStatusSchema>

export const TunnelHealthEventSchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  from_status: TunnelHealthSeverityEnum,
  to_status: TunnelHealthSeverityEnum,
  evidence: z.string(),
  detected_at: z.string(),
  notified_at: z.string().nullable().optional(),
})
export const TunnelHealthEventsResponseSchema = createApiResponseSchema(
  z.array(TunnelHealthEventSchema)
)
export type TunnelHealthEvent = z.infer<typeof TunnelHealthEventSchema>

export const TunnelActionLogSchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  level: z.string(),
  command_description: z.string(),
  command_detail: z.string(),
  simulated: z.boolean(),
  result: z.string(),
  executed_at: z.string(),
})
export const TunnelActionLogsResponseSchema = createApiResponseSchema(
  z.array(TunnelActionLogSchema)
)
export type TunnelActionLog = z.infer<typeof TunnelActionLogSchema>

export const TunnelHealthScoreSchema = z.object({
  score: z.number(),
  sampled_at: z.string(),
})
export const TunnelHealthScoresResponseSchema = createApiResponseSchema(
  z.array(TunnelHealthScoreSchema)
)
export type TunnelHealthScorePoint = z.infer<typeof TunnelHealthScoreSchema>

export const TunnelIncidentDiagnosisSchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  cause: z.enum([
    'remote_unreachable',
    'router_uplink_down',
    'router_resource_exhausted',
    'unknown',
  ]),
  evidence: z.string(),
  diagnosed_at: z.string(),
})
export const TunnelIncidentDiagnosesResponseSchema = createApiResponseSchema(
  z.array(TunnelIncidentDiagnosisSchema)
)
export type TunnelIncidentDiagnosis = z.infer<
  typeof TunnelIncidentDiagnosisSchema
>

export const TunnelBackupProbeResultSchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  backup_gateway_ip: z.string(),
  reachable: z.boolean(),
  evidence: z.string(),
  probed_at: z.string(),
})
export const TunnelBackupProbeResultsResponseSchema = createApiResponseSchema(
  z.array(TunnelBackupProbeResultSchema)
)
export type TunnelBackupProbeResult = z.infer<
  typeof TunnelBackupProbeResultSchema
>

export const TunnelAiSettingsSchema = z.object({
  dry_run: z.boolean(),
  emergency_stop: z.boolean(),
})
export const TunnelAiSettingsResponseSchema = createApiResponseSchema(
  TunnelAiSettingsSchema
)
export type TunnelAiSettings = z.infer<typeof TunnelAiSettingsSchema>

export const TunnelPolicyDetectionSchema = z.object({
  ping_fail_threshold: z.number(),
  tx_rx_asymmetry_window_minutes: z.number(),
  tx_rx_asymmetry_ratio: z.number(),
})
export const TunnelPolicyLevel1Schema = z.object({
  action: z.string(),
  wait_seconds: z.number(),
})
export const TunnelPolicyLevel2Schema = z.object({
  backup_target: z.string(),
  backup_gateway_ip: z.string(),
})
export const TunnelPolicyLevel3Schema = z.object({
  enabled: z.boolean(),
  type: z.string(),
  remote_address: z.string(),
  local_address: z.string(),
  masquerade: z.boolean(),
  gateway_ip: z.string(),
  new_interface_name: z.string(),
})
export const TunnelPolicyFallbackSchema = z.object({
  stable_duration_seconds: z.number(),
})
export const TunnelPolicyAntiFlappingSchema = z.object({
  max_flaps: z.number(),
  window_minutes: z.number(),
  cooldown_hours: z.number(),
})
export const TunnelPolicySchema = z.object({
  id: z.number(),
  interface_name: z.string(),
  detection: TunnelPolicyDetectionSchema,
  level1: TunnelPolicyLevel1Schema,
  level2: TunnelPolicyLevel2Schema,
  level3: TunnelPolicyLevel3Schema,
  fallback: TunnelPolicyFallbackSchema,
  anti_flapping: TunnelPolicyAntiFlappingSchema,
})
export const TunnelPoliciesResponseSchema = createApiResponseSchema(
  z.array(TunnelPolicySchema)
)
export const TunnelPolicyResponseSchema = createApiResponseSchema(TunnelPolicySchema)
export type TunnelPolicy = z.infer<typeof TunnelPolicySchema>

export const RedlineEntrySchema = z.object({
  id: z.number(),
  kind: z.enum(['interface', 'ip', 'port']),
  value: z.string(),
  comment: z.string().nullable().optional(),
})
export const RedlineEntriesResponseSchema = createApiResponseSchema(
  z.array(RedlineEntrySchema)
)
export type RedlineEntry = z.infer<typeof RedlineEntrySchema>

export const GraphNodeSchema = z.object({
  id: z.number(),
  type: z.string(),
  mikrotik_ref_id: z.string(),
  name: z.string(),
  properties: z.string(),
})
export const GraphEdgeSchema = z.object({
  id: z.number(),
  from_node_id: z.number(),
  to_node_id: z.number(),
  relation: z.string(),
})
export const GraphSchema = z.object({
  snapshot: z.object({ id: z.number(), taken_at: z.string() }),
  nodes: z.array(GraphNodeSchema),
  edges: z.array(GraphEdgeSchema),
})
export const GraphResponseSchema = createApiResponseSchema(GraphSchema.nullable())
export type TunnelGraph = z.infer<typeof GraphSchema>
export type GraphNode = z.infer<typeof GraphNodeSchema>
export type GraphEdge = z.infer<typeof GraphEdgeSchema>

// --- Server-computed protocol/location/tunnel map (the admin's own
// explicit "خودش باید نقشه‌سازی کنه" requirement -- grouping is done by
// the backend, not reassembled client-side from a flat node/edge list). ---
export const NatRuleSummarySchema = z.object({
  comment: z.string(),
  port: z.string(),
  target: z.string(),
  mangle_routing_mark: z.string(),
  routing_table: z.string(),
})
export type NatRuleSummary = z.infer<typeof NatRuleSummarySchema>
export const TunnelMapTunnelSchema = z.object({
  interface_name: z.string(),
  interface_type: z.string(),
  nat_rules: z.array(NatRuleSummarySchema),
})
export const TunnelMapLocationGroupSchema = z.object({
  location: z.string(),
  tunnels: z.array(TunnelMapTunnelSchema),
})
export const TunnelMapProtocolGroupSchema = z.object({
  protocol: z.string(),
  locations: z.array(TunnelMapLocationGroupSchema),
})
export const TunnelMapSchema = z.object({
  snapshot_taken_at: z.string(),
  protocols: z.array(TunnelMapProtocolGroupSchema),
})
export const TunnelMapResponseSchema = createApiResponseSchema(
  TunnelMapSchema.nullable()
)
export type TunnelMap = z.infer<typeof TunnelMapSchema>
export type TunnelMapTunnel = z.infer<typeof TunnelMapTunnelSchema>
export type TunnelMapProtocolGroup = z.infer<typeof TunnelMapProtocolGroupSchema>

// --- User Manager protocol health (spec section ب-7, alert-only) ---
export const UserManagerProtocolHealthStatusSchema = z.object({
  protocol: z.string(),
  healthy: z.boolean(),
  router_enabled: z.boolean(),
  port_reachable: z.boolean(),
  last_checked_at: z.string(),
})
export const UserManagerProtocolHealthStatusesResponseSchema =
  createApiResponseSchema(z.array(UserManagerProtocolHealthStatusSchema))
export type UserManagerProtocolHealthStatus = z.infer<
  typeof UserManagerProtocolHealthStatusSchema
>

export const UserManagerProtocolHealthEventSchema = z.object({
  id: z.number(),
  protocol: z.string(),
  from_status: z.string(),
  to_status: z.string(),
  evidence: z.string(),
  detected_at: z.string(),
  notified_at: z.string().nullable().optional(),
})
export const UserManagerProtocolHealthEventsResponseSchema =
  createApiResponseSchema(z.array(UserManagerProtocolHealthEventSchema))
export type UserManagerProtocolHealthEvent = z.infer<
  typeof UserManagerProtocolHealthEventSchema
>
