import { createFileRoute } from '@tanstack/react-router'
import ApplicationLocations from '@/features/application-locations'

export const Route = createFileRoute('/_authenticated/application-locations/')(
  {
    component: ApplicationLocations,
  }
)
