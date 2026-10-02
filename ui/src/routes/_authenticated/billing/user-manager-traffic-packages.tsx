import { createFileRoute } from '@tanstack/react-router'
import UserManagerTrafficPackagesPage from '@/features/billing/user-manager-traffic-packages-page'

export const Route = createFileRoute(
  '/_authenticated/billing/user-manager-traffic-packages'
)({
  component: UserManagerTrafficPackagesPage,
})
