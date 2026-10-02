import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const ApiKeySchema = z.object({
  id: z.number(),
  label: z.string(),
  key_prefix: z.string(),
  last_used_at: z.number().optional(),
  revoked: z.boolean(),
  created_at: z.number(),
})
export type ApiKeyItem = z.infer<typeof ApiKeySchema>

export const ListApiKeysSchema = z.object({
  keys: z.array(ApiKeySchema),
})
export const ListApiKeysResponseSchema = createApiResponseSchema(ListApiKeysSchema)

export const CreateApiKeySchema = z.object({
  id: z.number(),
  key: z.string(),
  label: z.string(),
})
export const CreateApiKeyResponseSchema = createApiResponseSchema(CreateApiKeySchema)
export type CreateApiKeyResult = z.infer<typeof CreateApiKeySchema>
