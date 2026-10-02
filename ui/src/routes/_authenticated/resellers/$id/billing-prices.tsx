import { createFileRoute } from '@tanstack/react-router'
import ResellerBillingPricesPage from '@/features/billing/billing-prices-page'

export const Route = createFileRoute(
  '/_authenticated/resellers/$id/billing-prices'
)({
  component: ResellerBillingPricesPage,
})
