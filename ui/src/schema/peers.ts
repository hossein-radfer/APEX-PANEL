import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const PeerStatusEnum = z.enum([
  'active',
  'inactive',
  'expired',
  'suspended',
])

export const PeerSchema = z.object({
  id: z.number(),
  uuid: z.string(),
  disabled: z.boolean(),
  comment: z.string().nullable(),
  telegram_username: z.string().nullable(),
  name: z.string(),
  interface: z.string(),
  allowed_address: z.string(),
  dns_servers: z.string().optional().nullable(),
  traffic_limit: z.string().nullable(),
  expire_time: z.string().nullable(),
  download_bandwidth: z.string().nullable(),
  upload_bandwidth: z.string().nullable(),
  total_usage: z.string(),
  status: z.array(PeerStatusEnum),
  is_online: z.boolean(),
  is_shared: z.boolean(),
  reseller_id: z.number().nullable().optional(),
  reseller_name: z.string().nullable().optional(),
})

export const PeersSchema = z.array(PeerSchema).nullable()

export const PeerAllowedAddressSchema = z.object({
  allowed_address: z.string(),
})

export const PeerCredentialsSchema = z.object({
  private_key: z.string(),
  public_key: z.string(),
})

export const PeerShareSchema = z.object({
  is_shared: z.boolean(),
  uuid: z.string().nullable(),
  expire_time: z.string().nullable(),
})

export const PeerStatsSchema = z.object({
  name: z.string(),
  expire_time: z.string().nullable(),
  traffic_limit: z.string().nullable(),
  download_usage: z.string(),
  upload_usage: z.string(),
  total_usage: z.string(),
  usage_percent: z.string().nullable(),
  is_online: z.boolean(),
  location: z.string().nullable().optional(),
})

export const ResellerActivitySchema = z.object({
  reseller_id: z.number(),
  reseller_name: z.string(),
  online_peers: z.number(),
  total_peers: z.number(),
  today_usage_gb: z.string(),
})

export const ResellerActivitySummarySchema = z.object({
  resellers: z.array(ResellerActivitySchema),
  total_online_peers: z.number(),
  total_today_usage_gb: z.string(),
})

export const SelfActivitySchema = z.object({
  online_peers: z.number(),
  total_peers: z.number(),
  today_usage_gb: z.string(),
})

export const ResellerActivitySummaryResponseSchema = createApiResponseSchema(
  ResellerActivitySummarySchema
)
export const SelfActivityResponseSchema = createApiResponseSchema(
  SelfActivitySchema
)

export const PeerResponseSchema = createApiResponseSchema(PeerSchema)
export const PeersResponseSchema = createApiResponseSchema(PeersSchema)
export const PeerAllowedAddressResponseSchema = createApiResponseSchema(
  PeerAllowedAddressSchema
)
export const PeerCredentialsResponseSchema = createApiResponseSchema(
  PeerCredentialsSchema
)
export const PeerShareResponseSchema = createApiResponseSchema(PeerShareSchema)
export const PeerStatsResponseSchema = createApiResponseSchema(PeerStatsSchema)

export const FetchPeerAllowedAddressSchema = z.object({
  interface_id: z.number().int().positive(),
})

export const CreatePeerSchema = z.object({
  comment: z.string().optional().nullable(),
  telegram_username: z.string().optional().nullable(),
  name: z.string().min(1, 'Name is required'),
  interface_id: z.number().min(1, 'Interface ID is required'),
  private_key: z.string().min(1, 'Private Key is required'),
  public_key: z.string().min(1, 'Public Key is required'),
  allowed_address: z.string().min(1, 'Allowed Address is required'),
  dns_servers: z.string().optional().nullable(),
  preshared_key: z.string().optional().nullable(),
  persistent_keepalive: z.string().optional().nullable(),
  endpoint: z.string().min(1, 'Endpoint is required'),
  expire_time: z.string().optional().nullable(),
  traffic_limit: z.string().optional().nullable(),
  download_bandwidth: z.string().optional().nullable(),
  upload_bandwidth: z.string().optional().nullable(),
})

export const UpdatePeerSchema = z.object({
  id: z.number().int().positive(),
  disabled: z.boolean().optional(),
  comment: z.string().optional().nullable(),
  telegram_username: z.string().optional().nullable(),
  name: z.string().min(1, 'Name is required'),
  allowed_address: z.string().min(1, 'Allowed Address is required'),
  dns_servers: z.string().optional().nullable(),
  persistent_keepalive: z.string().optional().nullable(),
  expire_time: z.string().optional().nullable(),
  traffic_limit: z.string().optional().nullable(),
  download_bandwidth: z.string().optional().nullable(),
  upload_bandwidth: z.string().optional().nullable(),
})

export const UpdatePeerShareExpireSchema = z.object({
  id: z.number().int().positive(),
  expire_time: z.string().optional().nullable(),
})

export const BulkCreatePeerSchema = z.object({
  count: z.number().int().min(1).max(500),
  interface_id: z.number().int().positive(),
  endpoint: z.string().min(1),
  duration_days: z.number().int().positive(),
  traffic_limit: z.string().optional().nullable(),
  download_bandwidth: z.string().optional().nullable(),
  upload_bandwidth: z.string().optional().nullable(),
  dns_servers: z.string().optional().nullable(),
  persistent_keepalive: z.string().optional().nullable(),
})

export const BulkCreatePeerResultSchema = z.object({
  peers: PeersSchema,
})

export const BulkCreatePeerResponseSchema = createApiResponseSchema(
  BulkCreatePeerResultSchema
)

export type BulkCreatePeerRequest = z.infer<typeof BulkCreatePeerSchema>

export type Peer = z.infer<typeof PeerSchema>
export type PeerStatus = z.infer<typeof PeerStatusEnum>
export type FetchPeerAllowedAddress = z.infer<
  typeof FetchPeerAllowedAddressSchema
>
export type CreatePeerRequest = z.infer<typeof CreatePeerSchema>
export type UpdatePeerRequest = z.infer<typeof UpdatePeerSchema>
export type UpdatePeerShareExpireRequest = z.infer<
  typeof UpdatePeerShareExpireSchema
>
export type PeerAllowedAddress = z.infer<typeof PeerAllowedAddressSchema>
export type PeerCredentials = z.infer<typeof PeerCredentialsSchema>
export type PeerShare = z.infer<typeof PeerShareSchema>
export type PeerStats = z.infer<typeof PeerStatsSchema>
export type ResellerActivity = z.infer<typeof ResellerActivitySchema>
export type ResellerActivitySummary = z.infer<
  typeof ResellerActivitySummarySchema
>
export type SelfActivity = z.infer<typeof SelfActivitySchema>
