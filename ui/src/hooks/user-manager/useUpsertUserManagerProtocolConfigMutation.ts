import { useMutation, useQueryClient } from '@tanstack/react-query'
import { upsertUserManagerProtocolConfig } from '@/api/user-manager.ts'

export const useUpsertUserManagerProtocolConfigMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: upsertUserManagerProtocolConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['user_manager_protocol_configs'],
      })
    },
  })
}
