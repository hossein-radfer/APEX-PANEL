import { useQuery } from '@tanstack/react-query'
import { fetchDNSAccountShareStatus } from '@/api/dns-account.ts'

export function useDNSAccountShareQuery(
  id: number,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: ['dns_account_share', id],
    queryFn: () => fetchDNSAccountShareStatus(id),
    enabled: !!id && (options?.enabled ?? true),
  })
}
