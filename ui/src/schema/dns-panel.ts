import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const DNSPanelStatusEnum = z.enum(['active', 'error'])

export const DNSPanelSchema = z.object({
  id: z.number(),
  name: z.string(),
  sale_title: z.string(),
  api_base_url: z.string(),
  status: DNSPanelStatusEnum,
  last_error: z.string().nullable().optional(),
  last_synced_at: z.string().nullable().optional(),
})

export const DNSPanelsSchema = z.array(DNSPanelSchema).nullable()

export const CreateDNSPanelSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  sale_title: z.string().min(1, 'Sale title is required'),
  api_base_url: z.string().min(1, 'API base URL is required'),
  api_key: z.string().min(1, 'API key is required'),
})

export const UpdateDNSPanelSchema = z.object({
  id: z.number().int().positive(),
  name: z.string().min(1).optional(),
  sale_title: z.string().min(1).optional(),
  api_base_url: z.string().min(1).optional(),
  // Omitted entirely (undefined) means "leave the stored key unchanged" --
  // mirrors UpdateDNSPanelRequest.APIKey being a *string on the Go side.
  api_key: z.string().min(1).optional(),
})

// Fields required to test a not-yet-saved panel form -- mirrors
// CreateDNSPanelSchema minus name/sale_title, which the test call doesn't
// need to reach the doctor-dns API.
export const TestDNSPanelSchema = z.object({
  api_base_url: z.string().min(1, 'API base URL is required'),
  api_key: z.string().min(1, 'API key is required'),
})

// One selectable plan/template returned by the test-connection call --
// template selection itself happens on the DNS ACCOUNT form, not here;
// this panel form only shows the list read-only as confirmation the
// connection actually works.
export const DNSTemplateOptionSchema = z.object({
  id: z.number(),
  name: z.string(),
  is_default: z.boolean(),
})

export const DNSTestConnectionResultSchema = z.object({
  templates: z.array(DNSTemplateOptionSchema),
})

// Reseller-safe view of a registered DNS panel -- just enough to render a
// "which DNS panel" picker (id + name), unlike the full admin-only
// DNSPanel schema. See GET /dns-account/reseller/:reseller_id/panels/summary.
export const DNSPanelSummarySchema = z.object({
  id: z.number(),
  name: z.string(),
})
export const DNSPanelSummariesSchema = z.array(DNSPanelSummarySchema).nullable()

export const AssignedDNSPanelsSchema = z.array(z.number()).nullable()

export const AssignResellerDNSPanelsSchema = z.object({
  panel_ids: z.array(z.number()),
})

export const DNSPanelResponseSchema = createApiResponseSchema(DNSPanelSchema)
export const DNSPanelsResponseSchema = createApiResponseSchema(DNSPanelsSchema)
export const DNSTestConnectionResultResponseSchema = createApiResponseSchema(
  DNSTestConnectionResultSchema
)
export const DNSPanelSummariesResponseSchema = createApiResponseSchema(
  DNSPanelSummariesSchema
)
export const AssignedDNSPanelsResponseSchema = createApiResponseSchema(
  AssignedDNSPanelsSchema
)

export type DNSPanelStatus = z.infer<typeof DNSPanelStatusEnum>
export type DNSPanel = z.infer<typeof DNSPanelSchema>
export type CreateDNSPanelRequest = z.infer<typeof CreateDNSPanelSchema>
export type UpdateDNSPanelRequest = z.infer<typeof UpdateDNSPanelSchema>
export type TestDNSPanelRequest = z.infer<typeof TestDNSPanelSchema>
export type DNSTemplateOption = z.infer<typeof DNSTemplateOptionSchema>
export type DNSTestConnectionResult = z.infer<
  typeof DNSTestConnectionResultSchema
>
export type DNSPanelSummary = z.infer<typeof DNSPanelSummarySchema>
export type DNSPanelSummaries = z.infer<typeof DNSPanelSummariesSchema>
export type AssignedDNSPanels = z.infer<typeof AssignedDNSPanelsSchema>
export type AssignResellerDNSPanelsRequest = z.infer<
  typeof AssignResellerDNSPanelsSchema
>
