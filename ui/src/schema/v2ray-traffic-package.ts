import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// Mirrors user-manager-traffic-package.ts exactly, for the separate V2Ray
// traffic-package pool.

// price_amount is a whole-Toman integer -- see traffic-package.ts for the
// rationale.
export const V2RayTrafficPackageSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  is_active: z.boolean(),
})

export const V2RayTrafficPackagesSchema = z.array(V2RayTrafficPackageSchema)

export const CreateV2RayTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional().nullable(),
  traffic_bytes: z.number().positive('Traffic size must be positive'),
  price_amount: z.number().min(0, 'Price must be non-negative'),
  is_active: z.boolean().optional(),
})

export const UpdateV2RayTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  description: z.string().optional().nullable(),
  traffic_bytes: z
    .number()
    .positive('Traffic size must be positive')
    .optional(),
  price_amount: z.number().min(0, 'Price must be non-negative').optional(),
  is_active: z.boolean().optional(),
})

export const V2RayPackagePurchaseSchema = z.object({
  id: z.number(),
  traffic_package_name: z.string(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  created_at: z.number(),
})

export const V2RayPackagePurchasesSchema = z.array(V2RayPackagePurchaseSchema)

export const V2RayTrafficPackageResponseSchema = createApiResponseSchema(
  V2RayTrafficPackageSchema
)
export const V2RayTrafficPackagesResponseSchema = createApiResponseSchema(
  V2RayTrafficPackagesSchema
)
export const V2RayPackagePurchaseResponseSchema = createApiResponseSchema(
  V2RayPackagePurchaseSchema
)
export const V2RayPackagePurchasesResponseSchema = createApiResponseSchema(
  V2RayPackagePurchasesSchema
)

export type V2RayTrafficPackage = z.infer<typeof V2RayTrafficPackageSchema>
export type CreateV2RayTrafficPackageRequest = z.infer<
  typeof CreateV2RayTrafficPackageSchema
>
export type UpdateV2RayTrafficPackageRequest = z.infer<
  typeof UpdateV2RayTrafficPackageSchema
>
export type V2RayPackagePurchase = z.infer<typeof V2RayPackagePurchaseSchema>
