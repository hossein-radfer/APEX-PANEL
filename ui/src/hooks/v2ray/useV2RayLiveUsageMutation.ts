import { useMutation, useQueryClient } from '@tanstack/react-query'
import { fetchV2RayLiveUsage } from '@/api/v2ray.ts'

// On-demand only -- calls x-ui LIVE for every location of the given
// package (see V2RayPackageService.GetLiveUsage's own doc comment), never
// auto-fetched on mount. Invalidates the ordinary list queries afterward
// since GetLiveUsage refreshes each location's cached usage as a side
// effect, so the next render of the (still cache-backed) package tables
// should reflect it too.
export const useV2RayLiveUsageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: fetchV2RayLiveUsage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['v2ray_packages_list'] })
      queryClient.invalidateQueries({
        queryKey: ['v2ray_packages_list_by_reseller'],
      })
    },
  })
}
