import { useQuery } from '@tanstack/react-query'
import { fetchDNSPlans } from '@/api/dns-plan.ts'

// GET /api/dns-plan is admin-only server-side, same convention as
// useDNSPanelsListQuery. `enabled` defaults to true so every existing
// admin-only call site keeps working unchanged; callers reachable by a
// reseller (e.g. DNSAccountForm) MUST pass `enabled: false` for a reseller
// session, or this query 403s the instant it mounts -- see
// useDNSPanelsListQuery's own identical doc comment on the confirmed,
// reported bug this convention avoids.
export const useDNSPlansListQuery = (enabled = true) =>
  useQuery({
    queryKey: ['dns_plans_list'],
    queryFn: fetchDNSPlans,
    enabled,
  })
