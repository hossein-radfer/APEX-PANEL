import { useMutation, useQueryClient } from '@tanstack/react-query'
import { purchaseV2RayTrafficPackage } from '@/api/v2ray-traffic-packages.ts'

export const usePurchaseV2RayTrafficPackageMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: purchaseV2RayTrafficPackage,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my_v2ray_package_purchases'] })
      queryClient.invalidateQueries({ queryKey: ['reseller'] })
      queryClient.invalidateQueries({ queryKey: ['wallet_balance'] })
    },
  })
}
