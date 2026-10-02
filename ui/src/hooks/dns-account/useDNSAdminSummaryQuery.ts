import { useQuery } from '@tanstack/react-query'
import { fetchDNSAdminSummary } from '@/api/dns-account.ts'

export const useDNSAdminSummaryQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['dns_admin_summary'],
    queryFn: fetchDNSAdminSummary,
    enabled,
    refetchInterval: 30_000,
  })
