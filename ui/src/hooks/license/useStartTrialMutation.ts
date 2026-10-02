import { useMutation, useQueryClient } from '@tanstack/react-query'
import { startTrial } from '@/api/license.ts'

export const useStartTrialMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: startTrial,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['license_status'] })
      // Lifts the root-level lockdown guard immediately (see
      // routes/__root.tsx) instead of waiting up to 60s for its own
      // refetchInterval to notice the newly-activated state.
      queryClient.invalidateQueries({ queryKey: ['public_license_status'] })
    },
  })
}
