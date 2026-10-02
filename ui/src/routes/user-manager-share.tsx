import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import UserManagerAccountShare from '@/features/user-manager-share'

export const Route = createFileRoute('/user-manager-share')({
  component: UserManagerAccountShare,

  validateSearch: z.object({
    shareId: z.string().uuid(),
  }),
})
