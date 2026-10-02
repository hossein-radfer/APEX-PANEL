import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateV2RayPackageShareExpire } from '@/api/v2ray.ts'
import { UpdateV2RayPackageShareExpireRequest } from '@/schema/v2ray.ts'

export const useUpdateV2RayPackageShareExpireMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, UpdateV2RayPackageShareExpireRequest>({
    mutationFn: updateV2RayPackageShareExpire,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['v2ray_package_share', variables.id],
      })
    },
  })
}
