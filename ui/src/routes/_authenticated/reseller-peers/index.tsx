import { createFileRoute } from '@tanstack/react-router'
import ResellerPeers from '@/features/reseller-peers'

export const Route = createFileRoute('/_authenticated/reseller-peers/')({
  component: ResellerPeers,
})
