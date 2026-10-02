import { createFileRoute } from '@tanstack/react-router'
import ApplicationPlansPage from '@/features/application-plans'

export const Route = createFileRoute('/_authenticated/application-plans/')({
  component: ApplicationPlansPage,
})
