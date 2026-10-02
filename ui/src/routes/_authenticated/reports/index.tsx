import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import ReportsPage from '@/features/reports'

export const Route = createFileRoute('/_authenticated/reports/')({
  component: RouteComponent,

  validateSearch: z.object({
    range: z.enum(['today', '7d', '30d']).catch('7d'),
  }),
})

function RouteComponent() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  return (
    <ReportsPage
      search={search}
      onSearchChange={(next) => navigate({ search: next })}
    />
  )
}
