import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const GeoIPStatusSchema = z.object({
  city_loaded: z.boolean(),
  asn_loaded: z.boolean(),
})
export const GeoIPStatusResponseSchema = createApiResponseSchema(GeoIPStatusSchema)
export type GeoIPStatus = z.infer<typeof GeoIPStatusSchema>

export const GeoIPModeSchema = z.object({
  online_enabled: z.boolean(),
})
export const GeoIPModeResponseSchema = createApiResponseSchema(GeoIPModeSchema)
export type GeoIPMode = z.infer<typeof GeoIPModeSchema>

export const SecurityIdentitySchema = z.object({
  protocol: z.string(),
  identity: z.string(),
  last_ip_address: z.string(),
  last_country: z.string().nullable().optional(),
  last_city: z.string().nullable().optional(),
  last_lat: z.number().nullable().optional(),
  last_lon: z.number().nullable().optional(),
  last_asn: z.number().nullable().optional(),
  last_isp: z.string().nullable().optional(),
  last_connected_at: z.string(),
  is_currently_open: z.boolean(),
})
export const SecurityIdentitiesSchema = z.object({
  identities: z.array(SecurityIdentitySchema),
})
export const SecurityIdentitiesResponseSchema = createApiResponseSchema(
  SecurityIdentitiesSchema
)
export type SecurityIdentity = z.infer<typeof SecurityIdentitySchema>

export const SecuritySessionSchema = z.object({
  ip_address: z.string(),
  connected_at: z.string(),
  disconnected_at: z.string().nullable().optional(),
  country: z.string().nullable().optional(),
  region: z.string().nullable().optional(),
  city: z.string().nullable().optional(),
  lat: z.number().nullable().optional(),
  lon: z.number().nullable().optional(),
  timezone: z.string().nullable().optional(),
  asn: z.number().nullable().optional(),
  isp: z.string().nullable().optional(),
  used_bytes: z.number(),
})
export const SecurityHistorySchema = z.object({
  protocol: z.string(),
  identity: z.string(),
  sessions: z.array(SecuritySessionSchema),
})
export const SecurityHistoryResponseSchema = createApiResponseSchema(
  SecurityHistorySchema
)
export type SecuritySession = z.infer<typeof SecuritySessionSchema>
export type SecurityHistory = z.infer<typeof SecurityHistorySchema>

export const EtherTrafficRowSchema = z.object({
  src_address: z.string(),
  ip_protocol: z.string(),
  src_port: z.number().nullable().optional(),
  tx_bytes_per_second: z.number(),
  rx_bytes_per_second: z.number(),
  tx_packets_rate: z.number(),
  rx_packets_rate: z.number(),
  country: z.string().nullable().optional(),
  city: z.string().nullable().optional(),
  lat: z.number().nullable().optional(),
  lon: z.number().nullable().optional(),
  isp: z.string().nullable().optional(),
  sampled_at: z.string(),
})
export const EtherTrafficSchema = z.object({
  flows: z.array(EtherTrafficRowSchema),
})
export const EtherTrafficResponseSchema = createApiResponseSchema(EtherTrafficSchema)
export type EtherTrafficRow = z.infer<typeof EtherTrafficRowSchema>

export const SecurityThreatSchema = z.object({
  kind: z.enum(['shared_account', 'multi_country', 'scanning']),
  subject: z.string(),
  detail: z.string(),
  severity: z.enum(['high', 'medium']),
  country: z.string().nullable().optional(),
  city: z.string().nullable().optional(),
  last_seen_at: z.string(),
  occurrence_count: z.number(),
})
export const SecurityThreatsSchema = z.object({
  threats: z.array(SecurityThreatSchema),
})
export const SecurityThreatsResponseSchema = createApiResponseSchema(
  SecurityThreatsSchema
)
export type SecurityThreat = z.infer<typeof SecurityThreatSchema>

export const RetentionCleanupResultSchema = z.object({
  deleted_count: z.number(),
})
export const RetentionCleanupResponseSchema = createApiResponseSchema(
  RetentionCleanupResultSchema
)

export interface RetentionCleanupRequest {
  target: 'connection_log' | 'ether_traffic'
  older_than_days?: number
  inactive_users_only?: boolean
}
