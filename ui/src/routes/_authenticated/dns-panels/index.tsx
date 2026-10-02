import { createFileRoute } from '@tanstack/react-router'
import DNSPanels from '@/features/dns-panels'

export const Route = createFileRoute('/_authenticated/dns-panels/')({
  component: DNSPanels,
})
