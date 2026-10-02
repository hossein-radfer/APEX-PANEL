import { useMutation } from '@tanstack/react-query'
import { exportV2RayPackages } from '@/api/v2ray.ts'

export const useExportV2RayPackagesMutation = () => {
  return useMutation({
    mutationFn: exportV2RayPackages,
  })
}
