import { useMutation, useQueryClient } from '@tanstack/react-query'
import { renewV2RayPackage } from '@/api/v2ray.ts'

export const useRenewV2RayPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: renewV2RayPackage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_packages_list'] })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller'],
      })
    },
  })
}
