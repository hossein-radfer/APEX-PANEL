import {
  GenerateSupportTokenResponseSchema,
  GenerateSupportTokenResult,
  SupportTokenStatus,
  SupportTokenStatusResponseSchema,
} from '@/schema/support-token.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchSupportTokenStatus = async (): Promise<SupportTokenStatus> => {
  const { data } = await axiosInstance.get('/support/token')
  const parsed = SupportTokenStatusResponseSchema.parse(data)
  return parsed.data
}

export const generateSupportToken = async (): Promise<GenerateSupportTokenResult> => {
  const { data } = await axiosInstance.post('/support/token')
  const parsed = GenerateSupportTokenResponseSchema.parse(data)
  return parsed.data
}

export const revokeSupportToken = async (): Promise<void> => {
  await axiosInstance.delete('/support/token')
}
