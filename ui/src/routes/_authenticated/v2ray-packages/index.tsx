import { createFileRoute } from '@tanstack/react-router'
import V2RayPackages from '@/features/v2ray-packages'

export const Route = createFileRoute('/_authenticated/v2ray-packages/')({
  component: V2RayPackages,
})
