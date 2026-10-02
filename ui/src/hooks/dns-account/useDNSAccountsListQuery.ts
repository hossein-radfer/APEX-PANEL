import { useQuery } from '@tanstack/react-query'
import { fetchDNSAccountsList } from '@/api/dns-account.ts'

export const useDNSAccountsListQuery = () =>
  useQuery({
    queryKey: ['dns_accounts_list'],
    queryFn: fetchDNSAccountsList,
  })
