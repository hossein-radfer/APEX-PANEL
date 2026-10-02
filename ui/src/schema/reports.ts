import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// Shared range/protocol enums mirrored from the Go backend's
// schema/reports.go -- "range" is always one of these three strings, and
// "protocol" (where present) is one of the three provider keys or "" for
// "all protocols".
export const ReportsRangeEnum = z.enum(['today', '7d', '30d'])
export type ReportsRange = z.infer<typeof ReportsRangeEnum>

export const ReportsProtocolEnum = z.enum([
  'wireguard',
  'user_manager',
  'v2ray',
])
export type ReportsProtocol = z.infer<typeof ReportsProtocolEnum>

export const ReportsFinancialBucketEnum = z.enum(['day', 'week', 'month'])
export type ReportsFinancialBucket = z.infer<
  typeof ReportsFinancialBucketEnum
>

// --- Report 1: daily usage -------------------------------------------------

export const ReportsDailyUsagePointSchema = z.object({
  date: z.string(),
  wireguard_bytes: z.number(),
  user_manager_bytes: z.number(),
  v2ray_bytes: z.number(),
  total_bytes: z.number(),
})

export const ReportsDailyUsageSchema = z.object({
  points: z.array(ReportsDailyUsagePointSchema),
})

export const ReportsDailyUsageResponseSchema = createApiResponseSchema(
  ReportsDailyUsageSchema
)

export type ReportsDailyUsagePoint = z.infer<
  typeof ReportsDailyUsagePointSchema
>
export type ReportsDailyUsage = z.infer<typeof ReportsDailyUsageSchema>

// --- Report 2: peak usage ---------------------------------------------------

export const ReportsPeakUsagePointSchema = z.object({
  date: z.string(),
  bytes: z.number(),
})

export const ReportsHourlyUsagePointSchema = z.object({
  hour: z.number(),
  bytes: z.number(),
})

export const ReportsPeakUsageSchema = z.object({
  points: z.array(ReportsPeakUsagePointSchema),
  peak_date: z.string().nullable().optional(),
  peak_bytes: z.number(),
  hourly_points: z.array(ReportsHourlyUsagePointSchema),
  peak_hour: z.number().nullable().optional(),
  peak_hour_bytes: z.number(),
})

export const ReportsPeakUsageResponseSchema = createApiResponseSchema(
  ReportsPeakUsageSchema
)

export type ReportsPeakUsagePoint = z.infer<typeof ReportsPeakUsagePointSchema>
export type ReportsHourlyUsagePoint = z.infer<
  typeof ReportsHourlyUsagePointSchema
>
export type ReportsPeakUsage = z.infer<typeof ReportsPeakUsageSchema>

// --- Reports 3/4/5: ranking (resellers / users / reseller-activity) -------

export const ReportsRankingRowSchema = z.object({
  id: z.number(),
  name: z.string(),
  protocol: z.string().optional(),
  bytes: z.number(),
  package_count: z.number().optional(),
})

export const ReportsRankingSchema = z.object({
  rows: z.array(ReportsRankingRowSchema),
})

export const ReportsRankingResponseSchema =
  createApiResponseSchema(ReportsRankingSchema)

export type ReportsRankingRow = z.infer<typeof ReportsRankingRowSchema>
export type ReportsRanking = z.infer<typeof ReportsRankingSchema>

// --- Report 6: expiring soon ------------------------------------------------

export const ReportsExpiringEntitySchema = z.object({
  id: z.number(),
  name: z.string(),
  protocol: z.string(),
  reseller_id: z.number().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  days_remaining: z.number().nullable().optional(),
  usage_percent: z.number().nullable().optional(),
  remaining_bytes: z.number().nullable().optional(),
})

export const ReportsExpiringSchema = z.object({
  expiring_by_soon_date: z.array(ReportsExpiringEntitySchema),
  expiring_by_volume: z.array(ReportsExpiringEntitySchema),
})

export const ReportsExpiringResponseSchema = createApiResponseSchema(
  ReportsExpiringSchema
)

export type ReportsExpiringEntity = z.infer<typeof ReportsExpiringEntitySchema>
export type ReportsExpiring = z.infer<typeof ReportsExpiringSchema>

// --- Report 7: protocol share -----------------------------------------------

export const ReportsProtocolShareSchema = z.object({
  wireguard_bytes: z.number(),
  user_manager_bytes: z.number(),
  v2ray_bytes: z.number(),
})

export const ReportsProtocolShareResponseSchema = createApiResponseSchema(
  ReportsProtocolShareSchema
)

export type ReportsProtocolShare = z.infer<typeof ReportsProtocolShareSchema>

// --- Report 8: online users --------------------------------------------------

export const ReportsOnlineUserSchema = z.object({
  name: z.string(),
  protocol: z.string(),
  location: z.string().optional(),
})

export const ReportsOnlineUsersSchema = z.object({
  total_online: z.number(),
  users: z.array(ReportsOnlineUserSchema),
})

export const ReportsOnlineUsersResponseSchema = createApiResponseSchema(
  ReportsOnlineUsersSchema
)

export type ReportsOnlineUser = z.infer<typeof ReportsOnlineUserSchema>
export type ReportsOnlineUsers = z.infer<typeof ReportsOnlineUsersSchema>

// --- Report 9: panel health --------------------------------------------------

export const ReportsPanelHealthRowSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  last_synced_at: z.string().nullable().optional(),
  recent_error_count: z.number(),
  location_count: z.number(),
  recent_errors: z.array(z.string()).optional(),
})

export const ReportsPanelHealthSchema = z.object({
  rows: z.array(ReportsPanelHealthRowSchema),
})

export const ReportsPanelHealthResponseSchema = createApiResponseSchema(
  ReportsPanelHealthSchema
)

export type ReportsPanelHealthRow = z.infer<typeof ReportsPanelHealthRowSchema>
export type ReportsPanelHealth = z.infer<typeof ReportsPanelHealthSchema>

// --- Report 10: financial ----------------------------------------------------

export const ReportsFinancialPointSchema = z.object({
  bucket: z.string(),
  charge_amount: z.number(),
  debit_amount: z.number(),
})

export const ReportsFinancialByResellerSchema = z.object({
  reseller_id: z.number(),
  reseller_name: z.string(),
  total_amount: z.number(),
})

export const ReportsFinancialSchema = z.object({
  points: z.array(ReportsFinancialPointSchema),
  resellers: z.array(ReportsFinancialByResellerSchema),
})

export const ReportsFinancialResponseSchema = createApiResponseSchema(
  ReportsFinancialSchema
)

export type ReportsFinancialPoint = z.infer<typeof ReportsFinancialPointSchema>
export type ReportsFinancialByReseller = z.infer<
  typeof ReportsFinancialByResellerSchema
>
export type ReportsFinancial = z.infer<typeof ReportsFinancialSchema>

// --- Report 11: renewal rate -------------------------------------------------

export const ReportsRenewalRateSchema = z.object({
  total_expired: z.number(),
  renewed: z.number(),
  renewal_rate: z.number(),
})

export const ReportsRenewalRateResponseSchema = createApiResponseSchema(
  ReportsRenewalRateSchema
)

export type ReportsRenewalRate = z.infer<typeof ReportsRenewalRateSchema>

// --- Report 12: popular locations -------------------------------------------

export const ReportsPopularLocationSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  sales_count: z.number(),
  bytes: z.number(),
})

export const ReportsPopularLocationsSchema = z.object({
  rows: z.array(ReportsPopularLocationSchema),
})

export const ReportsPopularLocationsResponseSchema = createApiResponseSchema(
  ReportsPopularLocationsSchema
)

export type ReportsPopularLocation = z.infer<
  typeof ReportsPopularLocationSchema
>
export type ReportsPopularLocations = z.infer<
  typeof ReportsPopularLocationsSchema
>

// --- Report 13: anomaly alerts -----------------------------------------------

export const ReportsAnomalyAlertSchema = z.object({
  name: z.string(),
  protocol: z.string(),
  today_bytes: z.number(),
  average_bytes: z.number(),
  multiplier: z.number(),
})

export const ReportsAnomalySchema = z.object({
  alerts: z.array(ReportsAnomalyAlertSchema),
})

export const ReportsAnomalyResponseSchema =
  createApiResponseSchema(ReportsAnomalySchema)

export type ReportsAnomalyAlert = z.infer<typeof ReportsAnomalyAlertSchema>
export type ReportsAnomaly = z.infer<typeof ReportsAnomalySchema>

// --- Report 15: resource usage -----------------------------------------------

export const ReportsResourcePointSchema = z.object({
  timestamp: z.number(),
  cpu_load_percent: z.number(),
  memory_used_percent: z.number(),
})

export const ReportsResourceSchema = z.object({
  points: z.array(ReportsResourcePointSchema),
  peak_cpu_percent: z.number(),
  peak_memory_percent: z.number(),
  average_cpu_percent: z.number(),
  average_memory_percent: z.number(),
  lowest_cpu_percent: z.number(),
  lowest_memory_percent: z.number(),
})

export const ReportsResourceResponseSchema =
  createApiResponseSchema(ReportsResourceSchema)

export type ReportsResourcePoint = z.infer<typeof ReportsResourcePointSchema>
export type ReportsResource = z.infer<typeof ReportsResourceSchema>

// --- Reseller quota prediction (category 2 item 2) --------------------------
// Reseller-scoped only (not part of the admin-only reports set above) --
// "days until quota exhausted" per protocol, computed server-side from the
// last 7 days of usage. days_remaining/quota_bytes/remaining_bytes are all
// nullable: unlimited quota or zero recent usage both surface as null
// rather than a fabricated number.

export const ResellerQuotaPredictionProtocolSchema = z.object({
  protocol: z.string(),
  quota_bytes: z.number().nullable().optional(),
  used_bytes: z.number(),
  remaining_bytes: z.number().nullable().optional(),
  avg_daily_usage_bytes: z.number(),
  days_remaining: z.number().nullable().optional(),
})

export const ResellerQuotaPredictionSchema = z.object({
  protocols: z.array(ResellerQuotaPredictionProtocolSchema),
})

export const ResellerQuotaPredictionResponseSchema = createApiResponseSchema(
  ResellerQuotaPredictionSchema
)

export type ResellerQuotaPredictionProtocol = z.infer<
  typeof ResellerQuotaPredictionProtocolSchema
>
export type ResellerQuotaPrediction = z.infer<
  typeof ResellerQuotaPredictionSchema
>
