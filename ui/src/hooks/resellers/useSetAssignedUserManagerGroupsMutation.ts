import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAssignedUserManagerGroups } from '@/api/resellers.ts'

export const useSetAssignedUserManagerGroupsMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAssignedUserManagerGroups,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['reseller_assigned_user_manager_groups', variables.resellerId],
      })
    },
  })
}
