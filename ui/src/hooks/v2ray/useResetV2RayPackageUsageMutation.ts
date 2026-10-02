import { useMutation, useQueryClient } from '@tanstack/react-query'
import { resetV2RayPackageUsage } from '@/api/v2ray.ts'

export const useResetV2RayPackageUsageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: resetV2RayPackageUsage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_packages_list'] })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller'],
      })
    },
  })
}
