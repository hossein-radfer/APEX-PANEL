import {
  ApiKeyItem,
  CreateApiKeyResponseSchema,
  CreateApiKeyResult,
  ListApiKeysResponseSchema,
} from '@/schema/api-key.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchApiKeys = async (): Promise<ApiKeyItem[]> => {
  const { data } = await axiosInstance.get('/api-keys')
  const parsed = ListApiKeysResponseSchema.parse(data)
  return parsed.data.keys
}

export const createApiKey = async (label: string): Promise<CreateApiKeyResult> => {
  const { data } = await axiosInstance.post('/api-keys', { label })
  const parsed = CreateApiKeyResponseSchema.parse(data)
  return parsed.data
}

export const revokeApiKey = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/api-keys/${id}`)
}
