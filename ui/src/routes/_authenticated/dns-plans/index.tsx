import { createFileRoute } from '@tanstack/react-router'
import DNSPlansPage from '@/features/dns-plans'

export const Route = createFileRoute('/_authenticated/dns-plans/')({
  component: DNSPlansPage,
})
