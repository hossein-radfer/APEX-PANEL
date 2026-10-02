import { useQuery } from '@tanstack/react-query'
import { fetchActiveApplicationPlans } from '@/api/application-plan.ts'

// GET /api/application-plan is reachable by both admin and reseller
// sessions (unlike useDNSPlansListQuery's admin-only endpoint) -- see
// ApplicationPlanController.ListPlans' own doc comment. No `enabled` gate
// needed here since a reseller session never 403s on this call.
export const useApplicationPlansListQuery = () =>
  useQuery({
    queryKey: ['application_plans_list'],
    queryFn: fetchActiveApplicationPlans,
  })
