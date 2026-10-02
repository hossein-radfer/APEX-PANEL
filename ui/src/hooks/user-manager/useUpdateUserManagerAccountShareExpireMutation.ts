import { useMutation, useQueryClient } from '@tanstack/react-query'
import { updateUserManagerAccountShareExpire } from '@/api/user-manager.ts'
import { UpdateUserManagerAccountShareExpireRequest } from '@/schema/user-manager.ts'

export const useUpdateUserManagerAccountShareExpireMutation = () => {
  const queryClient = useQueryClient()

  return useMutation<void, unknown, UpdateUserManagerAccountShareExpireRequest>({
    mutationFn: updateUserManagerAccountShareExpire,
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_account_share', variables.id],
      })
    },
  })
}
