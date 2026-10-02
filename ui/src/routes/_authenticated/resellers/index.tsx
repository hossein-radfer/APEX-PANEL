import { createFileRoute } from '@tanstack/react-router'
import Resellers from '@/features/resellers'

export const Route = createFileRoute('/_authenticated/resellers/')({
  component: Resellers,
})
