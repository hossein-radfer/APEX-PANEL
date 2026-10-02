import { createFileRoute } from '@tanstack/react-router'
import Applications from '@/features/applications'

export const Route = createFileRoute('/_authenticated/applications/')({
  component: Applications,
})
