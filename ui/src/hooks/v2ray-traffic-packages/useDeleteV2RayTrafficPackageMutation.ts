import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteV2RayTrafficPackage } from '@/api/v2ray-traffic-packages.ts'

export const useDeleteV2RayTrafficPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteV2RayTrafficPackage,
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
