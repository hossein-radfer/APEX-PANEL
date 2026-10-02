import { useQuery } from '@tanstack/react-query'
import { fetchV2RayPackagesByReseller } from '@/api/v2ray.ts'

export const useV2RayPackagesByResellerQuery = (
  resellerId: number | undefined
) =>
  useQuery({
    queryKey: ['v2ray_packages_list_by_reseller', resellerId],
    queryFn: () => fetchV2RayPackagesByReseller(resellerId as number),
    enabled: !!resellerId,
  })
