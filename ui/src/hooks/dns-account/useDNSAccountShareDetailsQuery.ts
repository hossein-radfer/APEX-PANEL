import { useQuery } from '@tanstack/react-query'
import { fetchDNSAccountShareDetails } from '@/api/dns-account.ts'

export const useDNSAccountShareDetailsQuery = (uuid: string | undefined) =>
  useQuery({
    queryKey: ['dns_account_share_details', uuid],
    queryFn: () => fetchDNSAccountShareDetails(uuid as string),
    enabled: !!uuid,
  })
