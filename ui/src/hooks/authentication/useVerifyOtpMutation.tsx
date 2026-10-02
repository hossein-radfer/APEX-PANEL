import { useMutation, useQueryClient } from '@tanstack/react-query'
import { verifyOtp } from '@/api/authentication.ts'

export const useVerifyOtpMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: verifyOtp,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['device_data'] })
    },
  })
}
