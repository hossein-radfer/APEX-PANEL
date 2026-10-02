import { AuditLogEntry, AuditLogResponseSchema } from '@/schema/audit-log.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchRecentAuditLog = async (limit = 50): Promise<AuditLogEntry[]> => {
  const { data } = await axiosInstance.get('/audit-log', { params: { limit } })
  const parsed = AuditLogResponseSchema.parse(data)
  return parsed.data || []
}
