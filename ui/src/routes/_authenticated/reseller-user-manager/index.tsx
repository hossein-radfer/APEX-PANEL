import { createFileRoute } from '@tanstack/react-router'
import ResellerUserManager from '@/features/reseller-user-manager'

export const Route = createFileRoute('/_authenticated/reseller-user-manager/')({
  component: ResellerUserManager,
})
