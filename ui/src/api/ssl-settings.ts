import {
  SSLSettings,
  SSLSettingsResponseSchema,
  UpdateSSLSettingsRequest,
} from '@/schema/ssl-settings.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchSSLSettings = async (): Promise<SSLSettings> => {
  const { data } = await axiosInstance.get('/ssl-settings')
  const parsed = SSLSettingsResponseSchema.parse(data)
  return parsed.data
}

export const updateSSLSettings = async (
  req: UpdateSSLSettingsRequest
): Promise<SSLSettings> => {
  const { data } = await axiosInstance.put('/ssl-settings', req)
  const parsed = SSLSettingsResponseSchema.parse(data)
  return parsed.data
}
