import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const SupportTokenStatusSchema = z.object({
  active: z.boolean(),
  expires_at: z.string().optional(),
})

export const SupportTokenStatusResponseSchema = createApiResponseSchema(
  SupportTokenStatusSchema
)

export type SupportTokenStatus = z.infer<typeof SupportTokenStatusSchema>

export const GenerateSupportTokenSchema = z.object({
  token: z.string(),
  expires_at: z.string(),
})

export const GenerateSupportTokenResponseSchema = createApiResponseSchema(
  GenerateSupportTokenSchema
)

export type GenerateSupportTokenResult = z.infer<typeof GenerateSupportTokenSchema>
