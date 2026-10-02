import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateV2RayTrafficPackage } from '@/api/v2ray-traffic-packages.ts'

export const useUpdateV2RayTrafficPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateV2RayTrafficPackage,
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
