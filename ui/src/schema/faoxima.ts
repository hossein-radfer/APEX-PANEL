import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const FaoximaInstanceSchema = z.object({
  status: z.enum(['PROVISIONING', 'ENABLED', 'DISABLED', 'ERROR']),
  admin_chat_id: z.string(),
  error_message: z.string().nullable().optional(),
  created_at: z.string(),
})

export type FaoximaInstance = z.infer<typeof FaoximaInstanceSchema>

export const FaoximaInstanceResponseSchema = createApiResponseSchema(
  FaoximaInstanceSchema.nullable()
)

export interface ProvisionFaoximaRequest {
  bot_token: string
  admin_chat_id?: string
}

export interface UpdateFaoximaTokenRequest {
  bot_token: string
}

// FaoximaAdminInstanceSchema is the admin's own "مدیریت ربات ایکس
// نمایندگان" list view -- unlike FaoximaInstanceSchema (a reseller's own
// self-service status), this carries the reseller's identity and the
// billing-period fields the admin's own explicit request introduced.
export const FaoximaAdminInstanceSchema = z.object({
  reseller_id: z.number(),
  reseller_name: z.string(),
  status: z.enum(['PROVISIONING', 'ENABLED', 'DISABLED', 'ERROR']),
  error_message: z.string().nullable().optional(),
  billing_period_days: z.number().nullable().optional(),
  period_activated_at: z.string().nullable().optional(),
  period_expires_at: z.string().nullable().optional(),
  created_at: z.string(),
})
export type FaoximaAdminInstance = z.infer<typeof FaoximaAdminInstanceSchema>
export const FaoximaAdminInstancesResponseSchema = createApiResponseSchema(
  z.array(FaoximaAdminInstanceSchema)
)

export interface AdminEnableFaoximaRequest {
  period_days: number
}
