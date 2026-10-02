import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import V2RayPackageShare from '@/features/v2ray-share'

export const Route = createFileRoute('/v2ray-share')({
  component: V2RayPackageShare,

  validateSearch: z.object({
    shareId: z.string().uuid(),
  }),
})
