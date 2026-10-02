import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const V2RayPackageStatusEnum = z.enum([
  'active',
  'suspended',
  'expired',
])

export const V2RayPackageLocationStatusSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  enabled: z.boolean(),
  last_synced_at: z.string().nullable().optional(),
  last_sync_error: z.string().nullable().optional(),
  // had_wrong_flow/flow_repaired surface the one-time client-flow repair
  // pass's result -- had_wrong_flow=true means this specific customer's
  // client was actually broken (created with the old hardcoded
  // xtls-rprx-vision flow on a non-TLS/Reality inbound) and has since been
  // auto-repaired.
  had_wrong_flow: z.boolean().optional().default(false),
  flow_repaired: z.boolean().optional().default(false),
  // is_online mirrors the periodic sync job's panel-wide onlines poll --
  // a point-in-time snapshot as of the last sync tick, not a live value.
  is_online: z.boolean().optional().default(false),
})

export const V2RayPackageSchema = z.object({
  id: z.number(),
  uuid: z.string(),
  customer_label: z.string().nullable().optional(),
  // comment is an admin-only note, distinct from customer_label -- never
  // shown to the customer/inside their subscription, see the Go model's
  // own doc comment on model.V2RayPackage.Comment.
  comment: z.string().nullable().optional(),
  total_volume_bytes: z.number(),
  used_bytes: z.number(),
  duration_days: z.number(),
  start_at: z.string().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  status: V2RayPackageStatusEnum,
  is_shared: z.boolean(),
  locations: z.array(V2RayPackageLocationStatusSchema),
  reseller_id: z.number().nullable().optional(),
  reseller_name: z.string().nullable().optional(),
})

export const V2RayPackagesSchema = z.array(V2RayPackageSchema).nullable()

export const CreateV2RayPackageSchema = z.object({
  customer_label: z.string().optional().nullable(),
  comment: z.string().optional().nullable(),
  total_volume_bytes: z.number().positive('Total volume must be positive'),
  duration_days: z.number().int().positive('Duration must be positive'),
  // Empty/omitted means "every panel the caller is allowed to use" -- see
  // CreateV2RayPackageRequest.PanelIDs's Go doc comment for the full
  // smart-default rule this implements.
  panel_ids: z.array(z.number()).optional(),
})

export const UpdateV2RayPackageSchema = z.object({
  id: z.number().int().positive(),
  customer_label: z.string().optional().nullable(),
  comment: z.string().optional().nullable(),
  total_volume_bytes: z.number().positive().optional(),
  duration_days: z.number().int().positive().optional(),
  status: V2RayPackageStatusEnum.optional(),
  // Omitted entirely means "leave this package's panel locations
  // untouched" -- sending it (even as an empty array) REPLACES the full
  // location set. See UpdateV2RayPackageRequest.PanelIDs's own Go doc
  // comment for the reported gap this fixes (there was previously no way
  // to edit an existing package's locations at all).
  panel_ids: z.array(z.number()).optional(),
})

// --- Bulk create + export ---------------------------------------------------

export const BulkCreateV2RayPackageSchema = z.object({
  count: z.number().int().min(1).max(500),
  total_volume_bytes: z.number().positive(),
  duration_days: z.number().int().positive(),
  panel_ids: z.array(z.number()).optional(),
})

export const BulkCreateV2RayPackageResultSchema = z.object({
  packages: V2RayPackagesSchema,
})

export const BulkCreateV2RayPackageResponseSchema = createApiResponseSchema(
  BulkCreateV2RayPackageResultSchema
)

export type BulkCreateV2RayPackageRequest = z.infer<
  typeof BulkCreateV2RayPackageSchema
>

export interface ExportV2RayPackagesRequest {
  package_ids: number[]
  include_share_link: boolean
  include_subscription_link: boolean
  base_url: string
  format: 'xlsx' | 'txt'
}

export const V2RayPackageShareSchema = z.object({
  is_shared: z.boolean(),
  uuid: z.string().nullable(),
  expire_time: z.string().nullable(),
})

export const UpdateV2RayPackageShareExpireSchema = z.object({
  id: z.number().int().positive(),
  expire_time: z.string().optional().nullable(),
})

export const V2RayShareLocationUsageSchema = z.object({
  title: z.string(),
  // The single raw vless://... link for this one panel -- NOT base64, the
  // literal URI a client app scans/pastes. Distinct from
  // subscription_url below, which is the combined multi-panel feed.
  config_link: z.string(),
  used_bytes: z.number(),
  is_online: z.boolean().optional().default(false),
  protocol: z.string().optional(),
})

// Public, unauthenticated share-page details -- served from
// GET /api/v2ray-share/:uuid. Layout: one box per entry in `locations`
// (title, config_link, the shared expire_at, that location's used_bytes,
// plus a client-side-rendered QR of config_link), then one final summary
// box (total_volume_bytes/used_bytes/usage_percent, is_online,
// subscription_url).
export const V2RayPackageShareDetailsSchema = z.object({
  subscription_url: z.string(),
  customer_label: z.string().nullable().optional(),
  status: z.string().optional().default('active'),
  total_volume_bytes: z.number(),
  used_bytes: z.number(),
  // The Go backend formats this as fmt.Sprintf("%.1f", percent) -- a JSON
  // string like "0.0", not a number. Parsing this as z.number() made
  // Zod's .parse() throw on every response (even 0%), which silently
  // failed the whole share-page query -- explaining why the QR code,
  // usage card, AND subscription URL all appeared blank together, not
  // just the usage percentage itself.
  usage_percent: z.string().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  days_remaining: z.number().nullable().optional(),
  is_online: z.boolean().optional().default(false),
  locations: z.array(V2RayShareLocationUsageSchema).optional().default([]),
})

// V2RayLiveUsageLocationSchema is one row of the on-demand "view live
// usage" table -- every field here comes from a getClientTraffics call
// made AT REQUEST TIME (GetLiveUsage), not the periodic sync job's cache.
export const V2RayLiveUsageLocationSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  enabled: z.boolean(),
  up_bytes: z.number(),
  down_bytes: z.number(),
  total_bytes: z.number(),
  error: z.string().nullable().optional(),
})

export const V2RayLiveUsageSchema = z.object({
  total_volume_bytes: z.number(),
  total_used_bytes: z.number(),
  locations: z.array(V2RayLiveUsageLocationSchema),
})

export const V2RayLiveUsageResponseSchema =
  createApiResponseSchema(V2RayLiveUsageSchema)

export type V2RayLiveUsageLocation = z.infer<
  typeof V2RayLiveUsageLocationSchema
>
export type V2RayLiveUsage = z.infer<typeof V2RayLiveUsageSchema>

// V2RaySelfSummarySchema is a reseller's own V2Ray dashboard aggregate --
// mirrors UserManagerSelfSummarySchema's shape.
export const V2RaySelfSummarySchema = z.object({
  online_packages: z.number(),
  total_packages: z.number(),
  quota_bytes: z.number().nullable().optional(),
  used_bytes: z.number(),
  remaining_bytes: z.number().nullable().optional(),
  max_packages: z.number().nullable().optional(),
})

export const V2RayAdminSummaryPanelSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  location_count: z.number(),
  online_count: z.number(),
  has_recent_error: z.boolean(),
})

export const V2RayAdminSummarySchema = z.object({
  total_packages: z.number(),
  total_locations: z.number(),
  online_locations: z.number(),
  total_volume_bytes: z.number(),
  total_used_bytes: z.number(),
  panels: z.array(V2RayAdminSummaryPanelSchema),
})

export const V2RaySelfSummaryResponseSchema =
  createApiResponseSchema(V2RaySelfSummarySchema)
export const V2RayAdminSummaryResponseSchema = createApiResponseSchema(
  V2RayAdminSummarySchema
)

export type V2RaySelfSummary = z.infer<typeof V2RaySelfSummarySchema>
export type V2RayAdminSummaryPanel = z.infer<
  typeof V2RayAdminSummaryPanelSchema
>
export type V2RayAdminSummary = z.infer<typeof V2RayAdminSummarySchema>

export const V2RaySaleTitleSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  title: z.string(),
})

export const V2RaySaleTitlesSchema = z.array(V2RaySaleTitleSchema).nullable()

export const V2RaySaleTitleRequestSchema = z.object({
  panel_id: z.number().int().positive(),
  title: z.string().min(1, 'Title is required'),
})

export const AssignedXuiPanelsSchema = z.array(z.number()).nullable()

// Reseller-safe view of an assigned panel -- just enough to render a
// "which server(s)" picker (id + name), unlike the full admin-only
// XuiPanel schema which also carries connection credentials. See
// GET /v2ray-package/reseller/:reseller_id/panels/summary.
export const XuiPanelSummarySchema = z.object({
  id: z.number(),
  name: z.string(),
})
export const AssignedXuiPanelSummariesSchema = z
  .array(XuiPanelSummarySchema)
  .nullable()

export const AssignResellerXuiPanelsSchema = z.object({
  panel_ids: z.array(z.number()),
})

export const V2RayPackageResponseSchema =
  createApiResponseSchema(V2RayPackageSchema)
export const V2RayPackagesResponseSchema = createApiResponseSchema(
  V2RayPackagesSchema
)
export const V2RayPackageShareResponseSchema = createApiResponseSchema(
  V2RayPackageShareSchema
)
export const V2RayPackageShareDetailsResponseSchema = createApiResponseSchema(
  V2RayPackageShareDetailsSchema
)
export const V2RaySaleTitlesResponseSchema = createApiResponseSchema(
  V2RaySaleTitlesSchema
)
export const V2RaySaleTitleResponseSchema =
  createApiResponseSchema(V2RaySaleTitleSchema)
export const AssignedXuiPanelsResponseSchema = createApiResponseSchema(
  AssignedXuiPanelsSchema
)
export const AssignedXuiPanelSummariesResponseSchema = createApiResponseSchema(
  AssignedXuiPanelSummariesSchema
)

export type V2RayPackageStatus = z.infer<typeof V2RayPackageStatusEnum>
export type V2RayPackageLocationStatus = z.infer<
  typeof V2RayPackageLocationStatusSchema
>
export type V2RayPackage = z.infer<typeof V2RayPackageSchema>
export type CreateV2RayPackageRequest = z.infer<
  typeof CreateV2RayPackageSchema
>
export type UpdateV2RayPackageRequest = z.infer<
  typeof UpdateV2RayPackageSchema
>
export type V2RayPackageShare = z.infer<typeof V2RayPackageShareSchema>
export type UpdateV2RayPackageShareExpireRequest = z.infer<
  typeof UpdateV2RayPackageShareExpireSchema
>
export type V2RayPackageShareDetails = z.infer<
  typeof V2RayPackageShareDetailsSchema
>
export type V2RayShareLocationUsage = z.infer<
  typeof V2RayShareLocationUsageSchema
>
export type V2RaySaleTitle = z.infer<typeof V2RaySaleTitleSchema>
export type V2RaySaleTitleRequest = z.infer<typeof V2RaySaleTitleRequestSchema>
export type AssignedXuiPanels = z.infer<typeof AssignedXuiPanelsSchema>
export type XuiPanelSummary = z.infer<typeof XuiPanelSummarySchema>
export type AssignedXuiPanelSummaries = z.infer<
  typeof AssignedXuiPanelSummariesSchema
>
export type AssignResellerXuiPanelsRequest = z.infer<
  typeof AssignResellerXuiPanelsSchema
>
