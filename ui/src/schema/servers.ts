import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const ServerStatusEnum = z.enum(['available', 'not_available'])

export const ServerSchema = z.object({
  id: z.number(),
  comment: z.string().nullable(),
  name: z.string(),
  ip_address: z.string(),
  api_port: z.string(),
  status: ServerStatusEnum,
  is_active: z.boolean(),
})

export const CreateServerSchema = z.object({
  comment: z.string().optional().nullable(),
  name: z.string().min(1, 'Name is required'),
  ip_address: z.string().min(1, 'IP Address is required'),
  api_port: z.string().min(1, 'Rest API Port is required'),
  is_ssl: z.boolean(),
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})

export const UpdateServerSchema = z.object({
  id: z.number().int().positive(),
  comment: z.string().optional().nullable(),
  name: z.string().optional().nullable(),
  ip_address: z.string().optional().nullable(),
  api_port: z.string().optional().nullable(),
  username: z.string().optional().nullable(),
  password: z.string().optional().nullable(),
})

export const ServersSchema = z.array(ServerSchema).nullable()

export const ServerResponseSchema = createApiResponseSchema(ServerSchema)
export const ServersResponseSchema = createApiResponseSchema(ServersSchema)

// Minimal, credential-free connection info. Available to any authenticated
// role, unlike ServerSchema which requires admin access.
export const ServerEndpointSchema = z.object({
  id: z.number(),
  name: z.string(),
  ip_address: z.string(),
})

export const ServerEndpointsSchema = z.array(ServerEndpointSchema).nullable()
export const ServerEndpointsResponseSchema = createApiResponseSchema(
  ServerEndpointsSchema
)

// On-demand live diagnostic of the configured Mikrotik server's
// reachability -- Reason is a short machine-readable code ("ok",
// "timeout", "connection_refused", "unauthorized", "bad_status",
// "not_configured", "db_error", "request_error", "connection_error");
// Detail is a human-readable elaboration (latency on success, the
// underlying error/HTTP status on failure).
export const ServerConnectionHealthSchema = z.object({
  connected: z.boolean(),
  reason: z.string(),
  detail: z.string(),
})
export const ServerConnectionHealthResponseSchema = createApiResponseSchema(
  ServerConnectionHealthSchema
)

export type Server = z.infer<typeof ServerSchema>
export type ServerStatus = z.infer<typeof ServerStatusEnum>
export type CreateServerRequest = z.infer<typeof CreateServerSchema>
export type UpdateServerRequest = Partial<Server> & { id: number }
export type ServerResponse = z.infer<typeof ServerResponseSchema>
export type ServersResponse = z.infer<typeof ServersResponseSchema>
export type ServerEndpoint = z.infer<typeof ServerEndpointSchema>
export type ServerConnectionHealth = z.infer<typeof ServerConnectionHealthSchema>
