import { useQuery } from '@tanstack/react-query'
import { fetchRecentAuditLog } from '@/api/audit-log.ts'

export const useAuditLogQuery = (enabled: boolean = true, limit = 50) =>
  useQuery({
    queryKey: ['audit_log', limit],
    queryFn: () => fetchRecentAuditLog(limit),
    enabled,
    refetchInterval: 30_000,
  })
