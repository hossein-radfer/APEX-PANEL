import { ListLogsResponseSchema, ListLogsResult } from '@/schema/log.ts'
import axiosInstance from '@/api/axios-instance.ts'

export interface LogFilterParams {
  level?: string
  logger?: string
  search?: string
  limit?: number
  includeRotated?: boolean
}

export const fetchLogs = async (
  filter: LogFilterParams = {}
): Promise<ListLogsResult> => {
  const { data } = await axiosInstance.get('/logs', {
    params: {
      level: filter.level || undefined,
      logger: filter.logger || undefined,
      search: filter.search || undefined,
      limit: filter.limit,
      include_rotated: filter.includeRotated ? 'true' : undefined,
    },
  })
  const parsed = ListLogsResponseSchema.parse(data)
  return parsed.data
}

export const fetchCurrentLogFile = async (): Promise<Blob> => {
  const response = await axiosInstance.get('/logs/download', {
    responseType: 'blob',
  })
  return response.data
}

export const fetchAllLogsArchive = async (): Promise<Blob> => {
  const response = await axiosInstance.get('/logs/download-all', {
    responseType: 'blob',
  })
  return response.data
}

export const clearLogs = async (): Promise<void> => {
  await axiosInstance.delete('/logs')
}
