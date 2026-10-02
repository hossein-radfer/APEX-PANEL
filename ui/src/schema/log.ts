import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const LogEntrySchema = z.object({
  timestamp: z.string(),
  level: z.string(),
  logger: z.string(),
  caller: z.string(),
  message: z.string(),
  fields: z.record(z.string(), z.unknown()).nullable().optional(),
})

export const ListLogsSchema = z.object({
  entries: z.array(LogEntrySchema),
  logger_names: z.array(z.string()),
})

export const ListLogsResponseSchema = createApiResponseSchema(ListLogsSchema)

export type LogEntry = z.infer<typeof LogEntrySchema>
export type ListLogsResult = z.infer<typeof ListLogsSchema>

export const LOG_LEVELS = ['debug', 'info', 'warn', 'error', 'dpanic', 'panic', 'fatal'] as const
