import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const InterfaceSchema = z.object({
  id: z.number(),
  server_id: z.number(),
  server_name: z.string(),
  interface_id: z.string(),
  disabled: z.boolean(),
  comment: z.string().nullable(),
  name: z.string(),
  listen_port: z.string(),
  mtu: z.string(),
  is_running: z.boolean(),
})

export const InterfacesSchema = z.array(InterfaceSchema).nullable()

export const CreateInterfaceSchema = z.object({
  // Optional while only a single-server install is common -- omitted
  // falls back to that one server server-side (see
  // WgInterface.CreateInterface); sent explicitly once more than one
  // server is registered.
  server_id: z.number().optional(),
  comment: z.string().optional().nullable(),
  name: z.string().min(1, 'Name is required'),
  listen_port: z.string().min(1, 'Listen Port is required'),
})

export const UpdateInterfaceSchema = z.object({
  id: z.number().int().positive(),
  disabled: z.boolean().optional(),
  comment: z.string().optional().nullable(),
  name: z.string().optional(),
})

export const InterfaceResponseSchema = createApiResponseSchema(InterfaceSchema)
export const InterfacesResponseSchema =
  createApiResponseSchema(InterfacesSchema)

export type Interface = z.infer<typeof InterfaceSchema>
export type CreateInterfaceRequest = z.infer<typeof CreateInterfaceSchema>
export type UpdateInterfaceRequest = Partial<Interface> & { id: number }
export type InterfaceResponse = z.infer<typeof InterfaceResponseSchema>
export type InterfacesResponse = z.infer<typeof InterfacesResponseSchema>
