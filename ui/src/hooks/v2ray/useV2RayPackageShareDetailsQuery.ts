import { useQuery } from '@tanstack/react-query'
import { fetchV2RayPackageShareDetails } from '@/api/v2ray.ts'

export const useV2RayPackageShareDetailsQuery = (uuid: string | undefined) =>
  useQuery({
    queryKey: ['v2ray_package_share_details', uuid],
    queryFn: () => fetchV2RayPackageShareDetails(uuid as string),
    enabled: !!uuid,
  })
