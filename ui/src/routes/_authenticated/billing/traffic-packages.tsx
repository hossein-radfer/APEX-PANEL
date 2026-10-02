import { createFileRoute } from '@tanstack/react-router'
import TrafficPackagesPage from '@/features/billing/traffic-packages-page'

export const Route = createFileRoute('/_authenticated/billing/traffic-packages')({
  component: TrafficPackagesPage,
})
