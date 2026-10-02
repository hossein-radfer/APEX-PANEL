import { useQuery } from '@tanstack/react-query'
import { fetchV2RayPackagesList } from '@/api/v2ray.ts'

export const useV2RayPackagesListQuery = () =>
  useQuery({
    queryKey: ['v2ray_packages_list'],
    queryFn: fetchV2RayPackagesList,
  })
