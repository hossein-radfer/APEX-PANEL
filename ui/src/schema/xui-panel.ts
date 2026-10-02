import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const XuiPanelStatusEnum = z.enum(['active', 'error'])

export const XuiPanelSchema = z.object({
  id: z.number(),
  name: z.string(),
  sale_title: z.string(),
  api_base_url: z.string(),
  username: z.string(),
  default_inbound_id: z.number(),
  protocol: z.string(),
  sub_base_url: z.string(),
  status: XuiPanelStatusEnum,
  last_error: z.string().nullable().optional(),
  last_synced_at: z.string().nullable().optional(),
  // container_server_id/container_server_name/container_name (item 9):
  // present only when the admin has mapped this panel to the RouterOS
  // container it's actually hosted in -- lets panel health checks tell
  // "the container stopped" apart from a generic connectivity failure.
  container_server_id: z.number().nullable().optional(),
  container_server_name: z.string().nullable().optional(),
  container_name: z.string().nullable().optional(),
})

export const XuiPanelsSchema = z.array(XuiPanelSchema).nullable()

export const CreateXuiPanelSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  sale_title: z.string().min(1, 'Sale title is required'),
  api_base_url: z.string().min(1, 'API base URL is required'),
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  // Must be > 0: the Go backend's CreateXuiPanelRequest tags this field
  // `validate:"required"`, and go-playground/validator's "required" rule
  // treats 0 (an int's zero value) as absent -- allowing 0 through here
  // would let the form submit before the admin has run "Test Connection"
  // and picked a real inbound, only to get rejected with a generic 400
  // instead of a clear inline "select an inbound" message.
  default_inbound_id: z
    .number()
    .int()
    .positive('Select an inbound (use Test Connection to load the list)'),
  protocol: z.string().min(1, 'Protocol is required'),
  sub_base_url: z.string().min(1, 'Subscription base URL is required'),
  // Both optional -- a panel not hosted inside a RouterOS container simply
  // omits this mapping, exactly like every panel created before item 9.
  container_server_id: z.number().int().positive().optional(),
  container_name: z.string().min(1).optional(),
})

export const UpdateXuiPanelSchema = z.object({
  id: z.number().int().positive(),
  name: z.string().min(1).optional(),
  sale_title: z.string().min(1).optional(),
  api_base_url: z.string().min(1).optional(),
  username: z.string().min(1).optional(),
  password: z.string().optional(),
  default_inbound_id: z.number().int().nonnegative().optional(),
  protocol: z.string().min(1).optional(),
  sub_base_url: z.string().min(1).optional(),
  container_server_id: z.number().int().positive().optional(),
  container_name: z.string().min(1).optional(),
  clear_container_mapping: z.boolean().optional(),
})

// Both the "test an already-saved panel" and "test in-progress form values"
// endpoints return this exact same shape.
export const XuiPanelInboundSchema = z.object({
  id: z.number(),
  remark: z.string(),
  protocol: z.string(),
})

export const XuiPanelTestResultSchema = z.object({
  inbounds: z.array(XuiPanelInboundSchema),
})

// Fields required to test a not-yet-saved panel form -- mirrors
// CreateXuiPanelSchema minus default_inbound_id, which is exactly what the
// test call is meant to help the admin choose.
export const TestXuiPanelSchema = z.object({
  api_base_url: z.string().min(1, 'API base URL is required'),
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
})

export const XuiPanelResponseSchema = createApiResponseSchema(XuiPanelSchema)
export const XuiPanelsResponseSchema = createApiResponseSchema(XuiPanelsSchema)
export const XuiPanelTestResultResponseSchema = createApiResponseSchema(
  XuiPanelTestResultSchema
)

export type XuiPanelStatus = z.infer<typeof XuiPanelStatusEnum>
export type XuiPanel = z.infer<typeof XuiPanelSchema>
export type CreateXuiPanelRequest = z.infer<typeof CreateXuiPanelSchema>
export type UpdateXuiPanelRequest = z.infer<typeof UpdateXuiPanelSchema>
export type XuiPanelInbound = z.infer<typeof XuiPanelInboundSchema>
export type XuiPanelTestResult = z.infer<typeof XuiPanelTestResultSchema>
export type TestXuiPanelRequest = z.infer<typeof TestXuiPanelSchema>
