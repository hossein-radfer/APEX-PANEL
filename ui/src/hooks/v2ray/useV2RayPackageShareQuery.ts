import { useQuery } from '@tanstack/react-query'
import { fetchV2RayPackageShareStatus } from '@/api/v2ray.ts'

export function useV2RayPackageShareQuery(
  id: number,
  options?: { enabled?: boolean }
) {
  return useQuery({
    queryKey: ['v2ray_package_share', id],
    queryFn: () => fetchV2RayPackageShareStatus(id),
    enabled: !!id && (options?.enabled ?? true),
  })
}
