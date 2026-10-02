import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateV2RayPackageShareStatus } from '@/api/v2ray.ts'

export const useUpdateV2RayPackageShareStatusMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, number>({
    mutationFn: updateV2RayPackageShareStatus,
    onSuccess: (_data, packageId) => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_package_share', packageId],
      })
    },
  })
}
