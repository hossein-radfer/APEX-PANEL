import { createFileRoute } from '@tanstack/react-router'
import ResellerWalletPage from '@/features/billing/wallet-page'

export const Route = createFileRoute('/_authenticated/billing/wallet')({
  component: ResellerWalletPage,
})
