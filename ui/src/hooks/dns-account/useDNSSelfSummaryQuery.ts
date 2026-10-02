import { useQuery } from '@tanstack/react-query'
import { fetchDNSSelfSummary } from '@/api/dns-account.ts'

export const useDNSSelfSummaryQuery = (enabled: boolean = true) =>
  useQuery({
    queryKey: ['dns_self_summary'],
    queryFn: fetchDNSSelfSummary,
    enabled,
    refetchInterval: 30_000,
  })
