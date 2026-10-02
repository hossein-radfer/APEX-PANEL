import { createFileRoute } from '@tanstack/react-router'
import ResellerDNS from '@/features/reseller-dns'

export const Route = createFileRoute('/_authenticated/reseller-dns/')({
  component: ResellerDNS,
})
