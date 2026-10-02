import { createFileRoute } from '@tanstack/react-router'
import ResellerV2Ray from '@/features/reseller-v2ray'

export const Route = createFileRoute('/_authenticated/reseller-v2ray/')({
  component: ResellerV2Ray,
})
