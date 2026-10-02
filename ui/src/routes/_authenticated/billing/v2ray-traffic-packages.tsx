import { createFileRoute } from '@tanstack/react-router'
import V2RayTrafficPackagesPage from '@/features/billing/v2ray-traffic-packages-page'

export const Route = createFileRoute(
  '/_authenticated/billing/v2ray-traffic-packages'
)({
  component: V2RayTrafficPackagesPage,
})
