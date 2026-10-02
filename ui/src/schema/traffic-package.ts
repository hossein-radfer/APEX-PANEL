import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// price_amount is a whole-Toman integer -- this panel bills exclusively in
// Toman (Iran's currency, no meaningful sub-unit in practical use), so
// there is no currency field/scaling here.
export const TrafficPackageSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  is_active: z.boolean(),
})

export const TrafficPackagesSchema = z.array(TrafficPackageSchema)

export const CreateTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional().nullable(),
  traffic_bytes: z.number().positive('Traffic size must be positive'),
  price_amount: z.number().min(0, 'Price must be non-negative'),
})

export const UpdateTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  description: z.string().optional().nullable(),
  traffic_bytes: z.number().positive('Traffic size must be positive').optional(),
  price_amount: z.number().min(0, 'Price must be non-negative').optional(),
  is_active: z.boolean().optional(),
})

export const PackagePurchaseSchema = z.object({
  id: z.number(),
  reseller_id: z.number(),
  traffic_package_id: z.number(),
  traffic_package_name: z.string(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  created_at: z.number(),
})

export const PackagePurchasesSchema = z.array(PackagePurchaseSchema)

export const TrafficPackageResponseSchema = createApiResponseSchema(TrafficPackageSchema)
export const TrafficPackagesResponseSchema = createApiResponseSchema(TrafficPackagesSchema)
export const PackagePurchaseResponseSchema = createApiResponseSchema(PackagePurchaseSchema)
export const PackagePurchasesResponseSchema = createApiResponseSchema(PackagePurchasesSchema)

export type TrafficPackage = z.infer<typeof TrafficPackageSchema>
export type CreateTrafficPackageRequest = z.infer<typeof CreateTrafficPackageSchema>
export type UpdateTrafficPackageRequest = z.infer<typeof UpdateTrafficPackageSchema>
export type PackagePurchase = z.infer<typeof PackagePurchaseSchema>
