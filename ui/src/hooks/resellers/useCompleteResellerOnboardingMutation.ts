import { useMutation, useQueryClient } from '@tanstack/react-query'
import { completeResellerOnboarding } from '@/api/resellers.ts'

export const useCompleteResellerOnboardingMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: completeResellerOnboarding,
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: ['resellers_list'] })
      queryClient.invalidateQueries({ queryKey: ['reseller', id] })
    },
  })
}
