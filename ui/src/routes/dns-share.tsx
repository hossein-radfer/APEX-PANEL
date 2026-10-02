import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import DNSAccountShare from '@/features/dns-share'

export const Route = createFileRoute('/dns-share')({
  component: DNSAccountShare,

  validateSearch: z.object({
    shareId: z.string().uuid(),
  }),
})
