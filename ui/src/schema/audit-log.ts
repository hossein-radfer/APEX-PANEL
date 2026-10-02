import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const AuditLogEntrySchema = z.object({
  id: z.number(),
  reseller_id: z.number(),
  reseller_name: z.string(),
  action: z.string(),
  description: z.string(),
  created_at: z.number(),
})

export const AuditLogEntriesSchema = z.array(AuditLogEntrySchema)
export const AuditLogResponseSchema = createApiResponseSchema(
  AuditLogEntriesSchema
)

export type AuditLogEntry = z.infer<typeof AuditLogEntrySchema>
