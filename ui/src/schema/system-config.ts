import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const PortConfigSchema = z.object({
  current_port: z.number(),
  pending_port: z.number().nullable(),
})

export const PortConfigResponseSchema = createApiResponseSchema(PortConfigSchema)

export type PortConfig = z.infer<typeof PortConfigSchema>

export interface UpdatePortConfigRequest {
  port: number
}

export const DatabaseSizeTableEntrySchema = z.object({
  name: z.string(),
  bytes_used: z.number(),
})

export const DatabaseSizeSchema = z.object({
  total_bytes: z.number(),
  tables: z.array(DatabaseSizeTableEntrySchema),
})

export const DatabaseSizeResponseSchema =
  createApiResponseSchema(DatabaseSizeSchema)

export type DatabaseSize = z.infer<typeof DatabaseSizeSchema>
export type DatabaseSizeTableEntry = z.infer<
  typeof DatabaseSizeTableEntrySchema
>

export const SystemHealthSchema = z.object({
  cpu_percent: z.number(),
  memory_total_bytes: z.number(),
  memory_used_bytes: z.number(),
  disk_total_bytes: z.number(),
  disk_used_bytes: z.number(),
  auto_backup_enabled: z.boolean(),
  last_backup_sent_date: z.string(),
  backup_is_up_to_date: z.boolean(),
})

export const SystemHealthResponseSchema =
  createApiResponseSchema(SystemHealthSchema)

export type SystemHealth = z.infer<typeof SystemHealthSchema>
