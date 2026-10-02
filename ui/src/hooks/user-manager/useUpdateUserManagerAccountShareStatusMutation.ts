import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccountShareStatus } from '@/api/user-manager.ts'

export const useUpdateUserManagerAccountShareStatusMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, number>({
    mutationFn: updateUserManagerAccountShareStatus,
    onSuccess: (_data, accountId) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_account_share', accountId],
      })
    },
  })
}
