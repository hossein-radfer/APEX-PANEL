import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const DNSAccountStatusEnum = z.enum(['active', 'suspended', 'expired'])

export const DNSAccountSchema = z.object({
  id: z.number(),
  uuid: z.string(),
  customer_label: z.string().nullable().optional(),
  // comment is an admin-only note, never shown to the customer -- same
  // convention as V2RayPackage.comment.
  comment: z.string().nullable().optional(),
  panel_id: z.number(),
  panel_name: z.string(),
  total_volume_bytes: z.number(),
  used_bytes: z.number(),
  speed_kbps: z.number(),
  duration_days: z.number(),
  start_at: z.string().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  template_id: z.number().nullable().optional(),
  status: DNSAccountStatusEnum,
  is_shared: z.boolean(),
  max_concurrent_ips: z.number(),
  daily_ip_registration_limit: z.number(),
  allowed_countries: z.array(z.string()).nullable().optional().default([]),
  plan_id: z.number().nullable().optional(),
  plan_name: z.string().nullable().optional(),
  current_ip: z.string().nullable().optional(),
  // is_online is a coarse proxy ("has a registered IP and is active"), NOT
  // a real-time connection signal -- labeled in the UI as "IP ثبت‌شده" (IP
  // registered), never "آنلاین" (online), and rendered with a muted/
  // neutral badge rather than a bright green "online now" pulse, per the
  // backend's own explicit honesty requirement for this field.
  is_online: z.boolean(),
  last_sync_error: z.string().nullable().optional(),
  reseller_id: z.number().nullable().optional(),
  reseller_name: z.string().nullable().optional(),
})

export const DNSAccountsSchema = z.array(DNSAccountSchema).nullable()

export const CreateDNSAccountSchema = z.object({
  customer_label: z.string().optional().nullable(),
  comment: z.string().optional().nullable(),
  panel_id: z.number().int().positive('Select a DNS panel'),
  total_volume_bytes: z.number().nonnegative(),
  speed_kbps: z.number().int().nonnegative(),
  duration_days: z.number().int().nonnegative(),
  template_id: z.number().int().optional().nullable(),
  max_concurrent_ips: z.number().int().min(1, 'Must be at least 1'),
  daily_ip_registration_limit: z.number().int().nonnegative(),
  allowed_countries: z.array(z.string()).optional(),
  // Local Apex-side pricing plan -- when set, any of the limit fields above
  // left at their zero-value get backfilled from this plan server-side. An
  // explicitly non-zero field always wins over the plan. See DNSPlan's own
  // doc comment for why this is separate from template_id (the doctor-dns
  // remote plan).
  plan_id: z.number().int().positive().optional().nullable(),
})

export const UpdateDNSAccountSchema = z.object({
  id: z.number().int().positive(),
  customer_label: z.string().optional().nullable(),
  comment: z.string().optional().nullable(),
  total_volume_bytes: z.number().nonnegative().optional(),
  speed_kbps: z.number().int().nonnegative().optional(),
  duration_days: z.number().int().nonnegative().optional(),
  template_id: z.number().int().optional().nullable(),
  status: DNSAccountStatusEnum.optional(),
  max_concurrent_ips: z.number().int().min(1).optional(),
  daily_ip_registration_limit: z.number().int().nonnegative().optional(),
  // Omitted entirely means "leave the current set untouched" -- sending it
  // (even as an empty array) REPLACES the full allowed-countries set, per
  // UpdateDNSAccountRequest.AllowedCountries's own doc comment.
  allowed_countries: z.array(z.string()).optional(),
  plan_id: z.number().int().positive().optional().nullable(),
})

export const DNSAccountShareStatusSchema = z.object({
  is_shared: z.boolean(),
  uuid: z.string().nullable().optional(),
  expire_time: z.string().nullable().optional(),
})

export const UpdateDNSAccountShareExpireSchema = z.object({
  id: z.number().int().positive(),
  expire_time: z.string().optional().nullable(),
})

// DNSSelfSummarySchema is a reseller's own DNS dashboard aggregate --
// mirrors V2RaySelfSummarySchema's shape.
export const DNSSelfSummarySchema = z.object({
  online_accounts: z.number(),
  total_accounts: z.number(),
  quota_bytes: z.number().nullable().optional(),
  used_bytes: z.number(),
  remaining_bytes: z.number().nullable().optional(),
  max_accounts: z.number().nullable().optional(),
})

export const AssignResellerDNSPanelsPutSchema = z.object({
  panel_ids: z.array(z.number()),
})

// DNSAdminSummaryPanel/DNSAdminSummarySchema mirror V2Ray's identical
// admin-dashboard rollup shape, simplified for DNS's one-account-per-panel
// model (no per-location fan-out to aggregate).
export const DNSAdminSummaryPanelSchema = z.object({
  panel_id: z.number(),
  panel_name: z.string(),
  account_count: z.number(),
  online_count: z.number(),
  has_recent_error: z.boolean(),
})

export const DNSAdminSummarySchema = z.object({
  total_accounts: z.number(),
  online_accounts: z.number(),
  total_volume_bytes: z.number(),
  total_used_bytes: z.number(),
  panels: z.array(DNSAdminSummaryPanelSchema),
})

export const DNSAdminSummaryResponseSchema =
  createApiResponseSchema(DNSAdminSummarySchema)

export type DNSAdminSummaryPanel = z.infer<typeof DNSAdminSummaryPanelSchema>
export type DNSAdminSummary = z.infer<typeof DNSAdminSummarySchema>

// --- Public, unauthenticated share/status page -----------------------------

export const DNSAccountShareDetailsSchema = z.object({
  customer_label: z.string().nullable().optional(),
  plan_title: z.string(),
  total_volume_bytes: z.number(),
  used_bytes: z.number(),
  // Formatted server-side as a string like "0.0" (same convention/gotcha
  // as V2RayPackageShareDetailsSchema.usage_percent) -- z.number() would
  // throw on parse for every response.
  usage_percent: z.string().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  days_remaining: z.number().nullable().optional(),
  status: z.string(),
  is_online: z.boolean(),
  current_ip: z.string().nullable().optional(),
  max_concurrent_ips: z.number(),
  daily_ip_registration_limit: z.number(),
  registrations_used_today: z.number(),
  allowed_countries: z.array(z.string()).nullable().optional().default([]),
  price_amount: z.number().nullable().optional(),
  speed_kbps: z.number().optional(),
})

export const RegisterDNSIPResultSchema = z.object({
  ok: z.boolean(),
  // Already localized (Persian) server-side -- displayed directly, never
  // re-worded client-side.
  message: z.string(),
  ip: z.string().optional(),
})

export const DNSAccountResponseSchema = createApiResponseSchema(DNSAccountSchema)
export const DNSAccountsResponseSchema =
  createApiResponseSchema(DNSAccountsSchema)
export const DNSAccountShareStatusResponseSchema = createApiResponseSchema(
  DNSAccountShareStatusSchema
)
export const DNSSelfSummaryResponseSchema =
  createApiResponseSchema(DNSSelfSummarySchema)
export const DNSAccountShareDetailsResponseSchema = createApiResponseSchema(
  DNSAccountShareDetailsSchema
)
export const RegisterDNSIPResultResponseSchema = createApiResponseSchema(
  RegisterDNSIPResultSchema
)

export type DNSAccountStatus = z.infer<typeof DNSAccountStatusEnum>
export type DNSAccount = z.infer<typeof DNSAccountSchema>
export type CreateDNSAccountRequest = z.infer<typeof CreateDNSAccountSchema>
export type UpdateDNSAccountRequest = z.infer<typeof UpdateDNSAccountSchema>
export type DNSAccountShareStatus = z.infer<typeof DNSAccountShareStatusSchema>
export type UpdateDNSAccountShareExpireRequest = z.infer<
  typeof UpdateDNSAccountShareExpireSchema
>
export type DNSSelfSummary = z.infer<typeof DNSSelfSummarySchema>
export type AssignResellerDNSPanelsRequest = z.infer<
  typeof AssignResellerDNSPanelsPutSchema
>
export type DNSAccountShareDetails = z.infer<
  typeof DNSAccountShareDetailsSchema
>
export type RegisterDNSIPResult = z.infer<typeof RegisterDNSIPResultSchema>
