import { createFileRoute } from '@tanstack/react-router'
import ApplicationManagement from '@/features/application-management'

export const Route = createFileRoute(
  '/_authenticated/application-management/'
)({
  component: ApplicationManagement,
})
