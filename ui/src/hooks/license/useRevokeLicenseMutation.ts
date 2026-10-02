import { useMutation, useQueryClient } from '@tanstack/react-query'
import { revokeLicense } from '@/api/license.ts'

export const useRevokeLicenseMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: revokeLicense,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['license_status'] })
      // Forces the root-level lockdown guard to reassert the activation
      // screen immediately, same as useActivateLicenseMutation does in
      // the opposite direction.
      queryClient.invalidateQueries({ queryKey: ['public_license_status'] })
    },
  })
}
