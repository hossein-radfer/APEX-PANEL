import { useQuery } from '@tanstack/react-query'
import { fetchAllV2RayTrafficPackages } from '@/api/v2ray-traffic-packages.ts'

export const useAllV2RayTrafficPackagesQuery = () =>
  useQuery({
    queryKey: ['v2ray_traffic_packages_all'],
    queryFn: fetchAllV2RayTrafficPackages,
  })
