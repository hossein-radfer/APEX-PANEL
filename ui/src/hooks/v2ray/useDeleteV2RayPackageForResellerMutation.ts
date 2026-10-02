import { useMutation, useQueryClient } from '@tanstack/react-query'
import { deleteV2RayPackageForReseller } from '@/api/v2ray.ts'

export const useDeleteV2RayPackageForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: deleteV2RayPackageForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller', variables.resellerId],
      })
    },
  })
}
