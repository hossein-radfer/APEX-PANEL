import { createFileRoute } from '@tanstack/react-router'
import Accounting from '@/features/accounting'

export const Route = createFileRoute('/_authenticated/accounting/')({
  component: Accounting,
})
