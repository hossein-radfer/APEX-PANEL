import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const LicenseStatusSchema = z.object({
  activated: z.boolean(),
  valid: z.boolean(),
  reason: z.string().optional(),
  plan_name: z.string().optional(),
  expires_at: z.string().optional(),
  server_count: z.number(),
  max_servers: z.number(),
  last_checked_at: z.string().optional(),
  in_grace_period: z.boolean(),
})

export const LicenseStatusResponseSchema = createApiResponseSchema(LicenseStatusSchema)

// PublicLicenseStatusSchema mirrors the backend's PublicLicenseStatusResponse
// -- the minimal, unauthenticated subset used by the root-level lockdown
// guard (see routes/__root.tsx) to decide whether to force the activation
// screen before any login attempt.
export const PublicLicenseStatusSchema = z.object({
  activated: z.boolean(),
  valid: z.boolean(),
  in_grace_period: z.boolean(),
  reason: z.string().optional(),
})

export const PublicLicenseStatusResponseSchema = createApiResponseSchema(
  PublicLicenseStatusSchema
)

export const UpdateCheckResultSchema = z.object({
  update_available: z.boolean(),
  version: z.string().optional(),
  change_log: z.string().optional(),
  force_update: z.boolean().optional(),
  current_version: z.string(),
})

export const UpdateCheckResponseSchema = createApiResponseSchema(UpdateCheckResultSchema)

export const ActivateLicenseSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  license_key: z.string().min(1, 'License key is required'),
})

export const StartTrialSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  email: z.string().min(1, 'Email is required').email('Enter a valid email address'),
})

export type LicenseStatus = z.infer<typeof LicenseStatusSchema>
export type PublicLicenseStatus = z.infer<typeof PublicLicenseStatusSchema>
export type UpdateCheckResult = z.infer<typeof UpdateCheckResultSchema>
export type ActivateLicenseRequest = z.infer<typeof ActivateLicenseSchema>
export type StartTrialRequest = z.infer<typeof StartTrialSchema>
