import { useQuery } from '@tanstack/react-query'
import { fetchActiveV2RayTrafficPackages } from '@/api/v2ray-traffic-packages.ts'

export const useActiveV2RayTrafficPackagesQuery = () =>
  useQuery({
    queryKey: ['v2ray_traffic_packages_active'],
    queryFn: fetchActiveV2RayTrafficPackages,
  })
