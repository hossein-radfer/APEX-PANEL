import {
  DatabaseSize,
  DatabaseSizeResponseSchema,
  PortConfig,
  PortConfigResponseSchema,
  SystemHealth,
  SystemHealthResponseSchema,
  UpdatePortConfigRequest,
} from '@/schema/system-config.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchPortConfig = async (): Promise<PortConfig> => {
  const { data } = await axiosInstance.get('/system-config/port')
  const parsed = PortConfigResponseSchema.parse(data)
  return parsed.data
}

export const updatePortConfig = async (
  req: UpdatePortConfigRequest
): Promise<PortConfig> => {
  const { data } = await axiosInstance.put('/system-config/port', req)
  const parsed = PortConfigResponseSchema.parse(data)
  return parsed.data
}

export const fetchDatabaseSize = async (): Promise<DatabaseSize> => {
  const { data } = await axiosInstance.get('/system-config/database-size')
  const parsed = DatabaseSizeResponseSchema.parse(data)
  return parsed.data
}

export const fetchSystemHealth = async (): Promise<SystemHealth> => {
  const { data } = await axiosInstance.get('/system-config/health')
  const parsed = SystemHealthResponseSchema.parse(data)
  return parsed.data
}
