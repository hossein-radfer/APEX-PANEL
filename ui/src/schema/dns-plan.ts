import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// price_amount is a whole-Toman integer, matching TrafficPackage's identical
// convention -- this panel bills exclusively in Toman.
export const DNSPlanSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable().optional(),
  price_amount: z.number(),
  total_volume_bytes: z.number(),
  speed_kbps: z.number(),
  duration_days: z.number(),
  max_concurrent_ips: z.number(),
  daily_ip_registration_limit: z.number(),
  is_active: z.boolean(),
})

export const DNSPlansSchema = z.array(DNSPlanSchema)

export const CreateDNSPlanSchema = z.object({
  name: z.string().min(1, 'نام الزامی است'),
  description: z.string().optional().nullable(),
  price_amount: z.number().min(0),
  total_volume_bytes: z.number().min(0),
  speed_kbps: z.number().min(0),
  duration_days: z.number().min(0),
  max_concurrent_ips: z.number().min(1),
  daily_ip_registration_limit: z.number().min(0),
})

export const UpdateDNSPlanSchema = z.object({
  name: z.string().min(1).optional(),
  description: z.string().optional().nullable(),
  price_amount: z.number().min(0).optional(),
  total_volume_bytes: z.number().min(0).optional(),
  speed_kbps: z.number().min(0).optional(),
  duration_days: z.number().min(0).optional(),
  max_concurrent_ips: z.number().min(1).optional(),
  daily_ip_registration_limit: z.number().min(0).optional(),
  is_active: z.boolean().optional(),
})

export const DNSPlanResponseSchema = createApiResponseSchema(DNSPlanSchema)
export const DNSPlansResponseSchema = createApiResponseSchema(DNSPlansSchema)

export type DNSPlan = z.infer<typeof DNSPlanSchema>
export type CreateDNSPlanRequest = z.infer<typeof CreateDNSPlanSchema>
export type UpdateDNSPlanRequest = z.infer<typeof UpdateDNSPlanSchema>
