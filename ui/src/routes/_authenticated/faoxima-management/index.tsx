import { createFileRoute } from '@tanstack/react-router'
import FaoximaManagement from '@/features/faoxima-management'

export const Route = createFileRoute(
  '/_authenticated/faoxima-management/'
)({
  component: FaoximaManagement,
})
