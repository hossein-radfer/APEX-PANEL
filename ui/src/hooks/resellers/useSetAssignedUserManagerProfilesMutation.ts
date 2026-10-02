import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAssignedUserManagerProfiles } from '@/api/resellers.ts'

export const useSetAssignedUserManagerProfilesMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAssignedUserManagerProfiles,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['reseller_assigned_user_manager_profiles', variables.resellerId],
      })
    },
  })
}
