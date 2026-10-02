import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import V2RaySubscriptionPage from '@/features/v2ray-sub'

export const Route = createFileRoute('/v2ray-sub')({
  component: V2RaySubscriptionPage,

  validateSearch: z.object({
    shareId: z.string().uuid().optional(),
  }),
})
