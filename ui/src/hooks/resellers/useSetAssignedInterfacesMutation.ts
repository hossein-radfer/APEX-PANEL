import { useMutation, useQueryClient } from '@tanstack/react-query'
import { setAssignedInterfaces } from '@/api/resellers.ts'

export const useSetAssignedInterfacesMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: setAssignedInterfaces,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['reseller_assigned_interfaces', variables.resellerId],
      })
    },
  })
}
