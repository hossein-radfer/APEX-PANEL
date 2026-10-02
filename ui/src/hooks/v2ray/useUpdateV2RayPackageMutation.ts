import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateV2RayPackage } from '@/api/v2ray.ts'

export const useUpdateV2RayPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateV2RayPackage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_packages_list'] })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller'],
      })
    },
  })
}
