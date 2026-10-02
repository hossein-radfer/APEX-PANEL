import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createV2RayPackageForReseller } from '@/api/v2ray.ts'

export const useCreateV2RayPackageForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: createV2RayPackageForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller', variables.resellerId],
      })
    },
  })
}
