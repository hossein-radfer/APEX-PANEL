import { useQuery } from '@tanstack/react-query'
import { fetchMyV2RayPackagePurchases } from '@/api/v2ray-traffic-packages.ts'

export const useMyV2RayPackagePurchasesQuery = () =>
  useQuery({
    queryKey: ['my_v2ray_package_purchases'],
    queryFn: fetchMyV2RayPackagePurchases,
  })
