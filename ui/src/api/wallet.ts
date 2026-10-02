import axiosInstance from '@/api/axios-instance.ts'
import { z } from 'zod'

// Schemas. Amounts are whole-Toman integers -- this panel bills
// exclusively in Toman (Iran's currency, no meaningful sub-unit in
// practical use), so there is no currency field/scaling here.
export const WalletSchema = z.object({
  reseller_id: z.number(),
  balance_amount: z.number(),
  is_frozen: z.boolean(),
  frozen_reason: z.string().nullable(),
})

export const LedgerEntrySchema = z.object({
  id: z.number(),
  reseller_id: z.number(),
  transaction_id: z.string(),
  entry_type: z.string(),
  amount: z.number(),
  balance_after: z.number(),
  description: z.string(),
  reference_type: z.string().nullable(),
  reference_id: z.number().nullable(),
  created_at: z.number(),
})

export const PricePlanSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable(),
  base_price_amount: z.number(),
  billing_interval: z.string(),
  traffic_allowance: z.number().nullable(),
  max_peers: z.number().nullable(),
  max_servers: z.number().nullable(),
  is_active: z.boolean(),
})

export const WalletResponseSchema = z.object({
  status: z.string(),
  status_code: z.number().optional(),
  data: z.object({
    reseller_id: z.number(),
    balance_amount: z.number(),
    is_frozen: z.boolean().optional(),
    frozen_reason: z.string().nullable().optional(),
  }),
})

export const LedgerResponseSchema = z.object({
  status: z.string(),
  status_code: z.number().optional(),
  data: z.array(LedgerEntrySchema),
})

export const PricePlansResponseSchema = z.object({
  status: z.string(),
  status_code: z.number().optional(),
  data: z.array(PricePlanSchema),
})

export type Wallet = z.infer<typeof WalletSchema>
export type LedgerEntry = z.infer<typeof LedgerEntrySchema>
export type PricePlan = z.infer<typeof PricePlanSchema>

// API functions
export const fetchWalletBalance = async (
  resellerID: number
): Promise<{
  balance_amount: number
  is_frozen?: boolean
  frozen_reason?: string | null
}> => {
  const { data } = await axiosInstance.get(`/wallet/reseller/${resellerID}/balance`)
  const parsed = WalletResponseSchema.parse(data)
  return parsed.data
}

export const fetchLedgerHistory = async (resellerID: number, limit = 50): Promise<LedgerEntry[]> => {
  const { data } = await axiosInstance.get(`/wallet/reseller/${resellerID}/ledger`, {
    params: { limit },
  })
  const parsed = LedgerResponseSchema.parse(data)
  return parsed.data
}

export const creditWallet = async (resellerID: number, amount: number, description: string): Promise<{ transaction_id: string; balance_after: number }> => {
  const { data } = await axiosInstance.post(`/wallet/reseller/${resellerID}/credit`, {
    amount,
    description,
  })
  return data.data
}

export const debitWallet = async (resellerID: number, amount: number, description: string): Promise<{ transaction_id: string; balance_after: number }> => {
  const { data } = await axiosInstance.post(`/wallet/reseller/${resellerID}/debit`, {
    amount,
    description,
  })
  return data.data
}

export const fetchPricePlans = async (): Promise<PricePlan[]> => {
  const { data } = await axiosInstance.get('/pricing/plan')
  const parsed = PricePlansResponseSchema.parse(data)
  return parsed.data
}
