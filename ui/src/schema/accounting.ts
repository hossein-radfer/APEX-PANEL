import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const AccountingPartnerSchema = z.object({
  id: z.number(),
  name: z.string(),
  comment: z.string().nullable().optional(),
})
export const AccountingPartnersResponseSchema = createApiResponseSchema(
  z.array(AccountingPartnerSchema)
)
export type AccountingPartner = z.infer<typeof AccountingPartnerSchema>

export const AccountingProtocolEnum = z.enum(['wireguard', 'user_manager', 'v2ray'])

export const AccountingCostSchema = z.object({
  id: z.number(),
  partner_id: z.number(),
  partner_name: z.string(),
  server_id: z.number().nullable().optional(),
  server_name: z.string().nullable().optional(),
  // manual_server_name: a free-text server label for infrastructure not
  // present in the managed Server table (e.g. a foreign VPS with no
  // RouterOS API) -- set only when server_id is null.
  manual_server_name: z.string().nullable().optional(),
  protocol: AccountingProtocolEnum.nullable().optional(),
  location_key: z.string().nullable().optional(),
  location_label: z.string().nullable().optional(),
  amount_toman: z.number(),
  description: z.string().nullable().optional(),
  paid_at: z.string().nullable().optional(),
  due_at: z.string().nullable().optional(),
  recurrence_interval_days: z.number().nullable().optional(),
  next_due_at: z.string().nullable().optional(),
})
export const AccountingCostsResponseSchema = createApiResponseSchema(
  z.array(AccountingCostSchema)
)
export type AccountingCost = z.infer<typeof AccountingCostSchema>

export const AccountingPaymentSchema = z.object({
  id: z.number(),
  customer_label: z.string(),
  amount_toman: z.number(),
  cost_toman: z.number(),
  profit_toman: z.number(),
  protocol: AccountingProtocolEnum.nullable().optional(),
  resource_id: z.number().nullable().optional(),
  location_key: z.string().nullable().optional(),
  location_label: z.string().nullable().optional(),
  note: z.string().nullable().optional(),
  has_receipt: z.boolean(),
  paid_at: z.string(),
})
export const AccountingPaymentsResponseSchema = createApiResponseSchema(
  z.array(AccountingPaymentSchema)
)
export type AccountingPayment = z.infer<typeof AccountingPaymentSchema>

export const AccountingPeriodSummarySchema = z.object({
  period: z.string(),
  income_toman: z.number(),
  cost_toman: z.number(),
})
export const AccountingSummarySchema = z.object({
  total_income_toman: z.number(),
  total_cost_toman: z.number(),
  profit_toman: z.number(),
  series: z.array(AccountingPeriodSummarySchema),
})
export const AccountingSummaryResponseSchema = createApiResponseSchema(
  AccountingSummarySchema
)
export type AccountingSummary = z.infer<typeof AccountingSummarySchema>

export const SellableResourceSchema = z.object({
  id: z.number(),
  name: z.string(),
})
export const SellableResourcesResponseSchema = createApiResponseSchema(
  z.array(SellableResourceSchema)
)
export type SellableResource = z.infer<typeof SellableResourceSchema>

export const UserProfitabilitySchema = z.object({
  payment_id: z.number(),
  customer_label: z.string(),
  protocol: AccountingProtocolEnum.nullable().optional(),
  resource_id: z.number().nullable().optional(),
  location_key: z.string().nullable().optional(),
  location_label: z.string().nullable().optional(),
  amount_toman: z.number(),
  cost_toman: z.number(),
  profit_toman: z.number(),
  paid_at: z.string(),
})
export const UserProfitabilityResponseSchema = createApiResponseSchema(
  z.array(UserProfitabilitySchema)
)
export type UserProfitability = z.infer<typeof UserProfitabilitySchema>

export const LocationProfitabilitySchema = z.object({
  protocol: z.string(),
  location_key: z.string(),
  location_label: z.string().nullable().optional(),
  income_toman: z.number(),
  cost_toman: z.number(),
  profit_toman: z.number(),
})
export const LocationProfitabilityResponseSchema = createApiResponseSchema(
  z.array(LocationProfitabilitySchema)
)
export type LocationProfitability = z.infer<typeof LocationProfitabilitySchema>

export interface CreatePartnerRequest {
  name: string
  comment?: string | null
}

export interface CreateCostRequest {
  partner_id: number
  server_id?: number | null
  manual_server_name?: string | null
  protocol?: string | null
  location_key?: string | null
  amount_toman: number
  description?: string | null
  paid_at?: string | null
  due_at?: string | null
  recurrence_interval_days?: number | null
}

export interface CreatePaymentRequest {
  customer_label: string
  amount_toman: number
  cost_toman?: number
  protocol?: string | null
  resource_id?: number | null
  note?: string | null
  paid_at: string
}
