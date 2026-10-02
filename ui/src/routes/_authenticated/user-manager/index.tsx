import { createFileRoute } from '@tanstack/react-router'
import UserManager from '@/features/user-manager'

export const Route = createFileRoute('/_authenticated/user-manager/')({
  component: UserManager,
})
