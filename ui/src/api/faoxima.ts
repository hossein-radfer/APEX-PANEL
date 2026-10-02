import {
  AdminEnableFaoximaRequest,
  FaoximaAdminInstance,
  FaoximaAdminInstancesResponseSchema,
  FaoximaInstance,
  FaoximaInstanceResponseSchema,
  ProvisionFaoximaRequest,
  UpdateFaoximaTokenRequest,
} from '@/schema/faoxima.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchFaoximaStatus = async (): Promise<FaoximaInstance | null> => {
  const { data } = await axiosInstance.get('/faoxima')
  const parsed = FaoximaInstanceResponseSchema.parse(data)
  return parsed.data
}

export const provisionFaoxima = async (
  req: ProvisionFaoximaRequest
): Promise<void> => {
  await axiosInstance.post('/faoxima/provision', req)
}

export const updateFaoximaToken = async (
  req: UpdateFaoximaTokenRequest
): Promise<void> => {
  await axiosInstance.put('/faoxima/token', req)
}

export const disableFaoxima = async (): Promise<void> => {
  await axiosInstance.post('/faoxima/disable')
}

export const enableFaoxima = async (): Promise<void> => {
  await axiosInstance.post('/faoxima/enable')
}

export const removeFaoxima = async (): Promise<void> => {
  await axiosInstance.delete('/faoxima')
}

export const downloadFaoximaBackup = async (): Promise<Blob> => {
  const { data } = await axiosInstance.get('/faoxima/backup', {
    responseType: 'blob',
  })
  return data
}

export const restoreFaoximaBackup = async (file: File): Promise<void> => {
  const formData = new FormData()
  formData.append('file', file)
  await axiosInstance.post('/faoxima/restore', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

// --- Admin-facing "مدیریت ربات ایکس نمایندگان" ---

export const fetchFaoximaAdminInstances = async (): Promise<
  FaoximaAdminInstance[]
> => {
  const { data } = await axiosInstance.get('/admin/faoxima')
  const parsed = FaoximaAdminInstancesResponseSchema.parse(data)
  return parsed.data || []
}

export const adminEnableFaoxima = async ({
  resellerId,
  ...req
}: AdminEnableFaoximaRequest & { resellerId: number }): Promise<void> => {
  await axiosInstance.post(`/admin/faoxima/${resellerId}/enable`, req)
}

export const adminDisableFaoxima = async (resellerId: number): Promise<void> => {
  await axiosInstance.post(`/admin/faoxima/${resellerId}/disable`)
}

export const adminResetFaoximaPeriod = async (
  resellerId: number
): Promise<void> => {
  await axiosInstance.post(`/admin/faoxima/${resellerId}/reset-period`)
}
