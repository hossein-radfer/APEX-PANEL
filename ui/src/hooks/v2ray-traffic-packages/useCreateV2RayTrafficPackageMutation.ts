import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createV2RayTrafficPackage } from '@/api/v2ray-traffic-packages.ts'

export const useCreateV2RayTrafficPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createV2RayTrafficPackage,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_traffic_packages_all'],
      })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_traffic_packages_active'],
      })
    },
  })
}
