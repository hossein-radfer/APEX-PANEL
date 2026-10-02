import {
  CreateServerRequest,
  CreateServerSchema,
  Server,
  ServerConnectionHealth,
  ServerConnectionHealthResponseSchema,
  ServerEndpoint,
  ServerEndpointsResponseSchema,
  ServerResponseSchema,
  ServersResponseSchema,
} from '@/schema/servers.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchServersList = async (): Promise<Server[]> => {
  const { data } = await axiosInstance.get('/server')
  const parsed = ServersResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchServerEndpoints = async (): Promise<ServerEndpoint[]> => {
  const { data } = await axiosInstance.get('/server/endpoints')
  const parsed = ServerEndpointsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchServerConnectionHealth =
  async (): Promise<ServerConnectionHealth> => {
    const { data } = await axiosInstance.get('/server/connection-health')
    const parsed = ServerConnectionHealthResponseSchema.parse(data)
    return parsed.data
  }

export const createServer = async (
  server: CreateServerRequest
): Promise<Server> => {
  const validated = CreateServerSchema.parse(server)
  const { data } = await axiosInstance.post('/server', validated)
  const parsed = ServerResponseSchema.parse(data)
  return parsed.data
}

export const updateServerStatus = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/server/${id}/status`)
}

export const updateServer = async (
  server: Partial<Server>
): Promise<Server> => {
  const { data } = await axiosInstance.put(`/server/${server.id}`, server)
  const parsed = ServerResponseSchema.parse(data)
  return parsed.data
}

export const deleteServer = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/server/${id}`)
}
