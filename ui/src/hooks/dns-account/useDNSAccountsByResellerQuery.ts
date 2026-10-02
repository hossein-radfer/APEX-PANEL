import { useQuery } from '@tanstack/react-query'
import { fetchDNSAccountsByReseller } from '@/api/dns-account.ts'

export const useDNSAccountsByResellerQuery = (
  resellerId: number | undefined
) =>
  useQuery({
    queryKey: ['dns_accounts_list_by_reseller', resellerId],
    queryFn: () => fetchDNSAccountsByReseller(resellerId as number),
    enabled: !!resellerId,
  })
