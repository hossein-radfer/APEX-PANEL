import {
  AddExtraAdminChatIDRequest,
  BotSettings,
  BotSettingsResponseSchema,
  ExtraAdminChatID,
  ExtraAdminChatIDsResponseSchema,
  TestSocks5ApiResponseSchema,
  TestSocks5Request,
  TestSocks5Result,
  UpdateBotSettingsRequest,
} from '@/schema/bot-settings.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchBotSettings = async (): Promise<BotSettings> => {
  const { data } = await axiosInstance.get('/bot-settings')
  const parsed = BotSettingsResponseSchema.parse(data)
  return parsed.data
}

export const updateBotSettings = async (
  req: UpdateBotSettingsRequest
): Promise<BotSettings> => {
  const { data } = await axiosInstance.put('/bot-settings', req)
  const parsed = BotSettingsResponseSchema.parse(data)
  return parsed.data
}

export const fetchExtraAdminChatIDs = async (): Promise<ExtraAdminChatID[]> => {
  const { data } = await axiosInstance.get('/bot-settings/extra-admins')
  const parsed = ExtraAdminChatIDsResponseSchema.parse(data)
  return parsed.data || []
}

export const addExtraAdminChatID = async (
  req: AddExtraAdminChatIDRequest
): Promise<ExtraAdminChatID> => {
  const { data } = await axiosInstance.post('/bot-settings/extra-admins', req)
  return data.data
}

export const removeExtraAdminChatID = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/bot-settings/extra-admins/${id}`)
}

export const testSocks5Connection = async (
  req: TestSocks5Request
): Promise<TestSocks5Result> => {
  const { data } = await axiosInstance.post('/bot-settings/test-socks5', req)
  const parsed = TestSocks5ApiResponseSchema.parse(data)
  return parsed.data
}
