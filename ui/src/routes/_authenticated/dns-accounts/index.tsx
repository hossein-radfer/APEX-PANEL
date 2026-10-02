import { createFileRoute } from '@tanstack/react-router'
import DNSAccounts from '@/features/dns-accounts'

export const Route = createFileRoute('/_authenticated/dns-accounts/')({
  component: DNSAccounts,
})
