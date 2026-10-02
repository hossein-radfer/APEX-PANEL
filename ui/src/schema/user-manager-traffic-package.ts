import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

// Mirrors traffic-package.ts exactly, for the separate User Manager
// (L2TP/PPTP/SSTP/OpenVPN) traffic-package pool.

// price_amount is a whole-Toman integer -- see traffic-package.ts for the
// rationale.
export const UserManagerTrafficPackageSchema = z.object({
  id: z.number(),
  name: z.string(),
  description: z.string().nullable(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  is_active: z.boolean(),
})

export const UserManagerTrafficPackagesSchema = z.array(
  UserManagerTrafficPackageSchema
)

export const CreateUserManagerTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  description: z.string().optional().nullable(),
  traffic_bytes: z.number().positive('Traffic size must be positive'),
  price_amount: z.number().min(0, 'Price must be non-negative'),
})

export const UpdateUserManagerTrafficPackageSchema = z.object({
  name: z.string().min(1, 'Name is required').optional(),
  description: z.string().optional().nullable(),
  traffic_bytes: z
    .number()
    .positive('Traffic size must be positive')
    .optional(),
  price_amount: z.number().min(0, 'Price must be non-negative').optional(),
  is_active: z.boolean().optional(),
})

export const UserManagerPackagePurchaseSchema = z.object({
  id: z.number(),
  reseller_id: z.number(),
  traffic_package_id: z.number(),
  traffic_package_name: z.string(),
  traffic_bytes: z.number(),
  price_amount: z.number(),
  created_at: z.number(),
})

export const UserManagerPackagePurchasesSchema = z.array(
  UserManagerPackagePurchaseSchema
)

export const UserManagerTrafficPackageResponseSchema = createApiResponseSchema(
  UserManagerTrafficPackageSchema
)
export const UserManagerTrafficPackagesResponseSchema =
  createApiResponseSchema(UserManagerTrafficPackagesSchema)
export const UserManagerPackagePurchaseResponseSchema =
  createApiResponseSchema(UserManagerPackagePurchaseSchema)
export const UserManagerPackagePurchasesResponseSchema =
  createApiResponseSchema(UserManagerPackagePurchasesSchema)

export type UserManagerTrafficPackage = z.infer<
  typeof UserManagerTrafficPackageSchema
>
export type CreateUserManagerTrafficPackageRequest = z.infer<
  typeof CreateUserManagerTrafficPackageSchema
>
export type UpdateUserManagerTrafficPackageRequest = z.infer<
  typeof UpdateUserManagerTrafficPackageSchema
>
export type UserManagerPackagePurchase = z.infer<
  typeof UserManagerPackagePurchaseSchema
>
