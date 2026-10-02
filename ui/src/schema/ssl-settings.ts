import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const SSLSettingsSchema = z.object({
  domain: z.string(),
  certificate_path: z.string(),
  private_key_path: z.string(),
  enabled: z.boolean(),
})

export const SSLSettingsResponseSchema = createApiResponseSchema(SSLSettingsSchema)

export type SSLSettings = z.infer<typeof SSLSettingsSchema>

export interface UpdateSSLSettingsRequest {
  domain?: string
  certificate_path?: string
  private_key_path?: string
  enabled?: boolean
}
