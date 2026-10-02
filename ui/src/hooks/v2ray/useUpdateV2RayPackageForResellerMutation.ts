import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateV2RayPackageForReseller } from '@/api/v2ray.ts'

export const useUpdateV2RayPackageForResellerMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: updateV2RayPackageForReseller,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller', variables.resellerId],
      })
    },
  })
}
