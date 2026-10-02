import { createFileRoute } from '@tanstack/react-router'
import TunnelHealth from '@/features/tunnel-health'

export const Route = createFileRoute('/_authenticated/tunnel-health/')({
  component: TunnelHealth,
})
