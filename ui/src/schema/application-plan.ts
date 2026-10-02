import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// price_amount is a whole-Toman integer, matching DNSPlan/TrafficPackage's
// identical convention.
export const ApplicationPlanSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable().optional(),
  price_amount: z.number(),
  total_volume_bytes: z.number(),
  duration_days: z.number(),
  max_online_users: z.number(),
  download_speed_limit_mbps: z.number().nullable().optional(),
  upload_speed_limit_mbps: z.number().nullable().optional(),
  is_active: z.boolean(),
})

export const ApplicationPlansSchema = z.array(ApplicationPlanSchema)

export const CreateApplicationPlanSchema = z.object({
  name: z.string().min(1, 'نام الزامی است'),
  description: z.string().optional().nullable(),
  price_amount: z.number().min(0),
  total_volume_bytes: z.number().min(0),
  duration_days: z.number().min(0),
  max_online_users: z.number().min(1),
  download_speed_limit_mbps: z.number().min(1).optional().nullable(),
  upload_speed_limit_mbps: z.number().min(1).optional().nullable(),
})

export const UpdateApplicationPlanSchema = z.object({
  name: z.string().min(1).optional(),
  description: z.string().optional().nullable(),
  price_amount: z.number().min(0).optional(),
  total_volume_bytes: z.number().min(0).optional(),
  duration_days: z.number().min(0).optional(),
  max_online_users: z.number().min(1).optional(),
  download_speed_limit_mbps: z.number().min(1).optional().nullable(),
  upload_speed_limit_mbps: z.number().min(1).optional().nullable(),
  clear_speed_limits: z.boolean().optional(),
  is_active: z.boolean().optional(),
})

export const ApplicationPlanResponseSchema = createApiResponseSchema(ApplicationPlanSchema)
export const ApplicationPlansResponseSchema = createApiResponseSchema(ApplicationPlansSchema)

export type ApplicationPlan = z.infer<typeof ApplicationPlanSchema>
export type CreateApplicationPlanRequest = z.infer<typeof CreateApplicationPlanSchema>
export type UpdateApplicationPlanRequest = z.infer<typeof UpdateApplicationPlanSchema>
