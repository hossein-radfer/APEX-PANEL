import { useQuery } from '@tanstack/react-query'
import { fetchDNSPanelsList } from '@/api/dns-panel.ts'

// GET /api/dns-panel is admin-only server-side, same convention as
// useXuiPanelsListQuery. `enabled` defaults to true so every existing
// admin-only call site keeps working unchanged; callers reachable by a
// reseller (e.g. DNSAccountForm) MUST pass `enabled: false` for a reseller
// session, or this query 403s the instant it mounts.
export const useDNSPanelsListQuery = (enabled = true) =>
  useQuery({
    queryKey: ['dns_panels_list'],
    queryFn: fetchDNSPanelsList,
    enabled,
  })
