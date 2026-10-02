import { useMutation, useQueryClient } from '@tanstack/react-query'
import { bulkCreateV2RayPackages } from '@/api/v2ray.ts'

export const useBulkCreateV2RayPackagesMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: bulkCreateV2RayPackages,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_packages_list'] })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller'],
      })
    },
  })
}
