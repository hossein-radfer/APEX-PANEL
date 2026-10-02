import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const loginRequestSchema = z.object({
  username: z.string().min(1, { message: 'Please enter your username' }),
  password: z
    .string()
    .min(1, {
      message: 'Please enter your password',
    })
    .min(7, {
      message: 'Password must be at least 7 characters long',
    }),
})

// A response with otp_required=true carries ONLY otp_required + otp_token
// (see the Go LoginResponse's own doc comment) -- every other field is
// present as its zero value and must be ignored, so all the "normal login"
// fields below are optional/nullable rather than required.
export const loginResponseSchema = z.object({
  user_id: z.number().optional(),
  username: z.string().optional(),
  access_token: z.string().optional(),
  refresh_token: z.string().optional(),
  expires_in: z.number().optional(),
  role: z.string().optional().default('admin'),
  reseller_id: z.number().nullable().optional(),
  otp_required: z.boolean().optional(),
  otp_token: z.string().optional(),
})

export const loginResponse = createApiResponseSchema(loginResponseSchema)

export const verifyOtpRequestSchema = z.object({
  otp_token: z.string().min(1),
  code: z.string().min(1),
})

export type VerifyOtpRequest = z.infer<typeof verifyOtpRequestSchema>

export const updateProfileSchema = z.object({
  old_password: z.string().min(1, {
    message: 'Please enter your password',
  }),
  new_username: z.string().optional(),
  new_password: z
    .string()
    .min(8, { message: 'Password must be at least 8 characters long' })
    .optional()
    .or(z.literal('')),
})

export type LoginRequest = z.infer<typeof loginRequestSchema>
export type LoginResponse = z.infer<typeof loginResponseSchema>
export type UpdateProfileRequest = z.infer<typeof updateProfileSchema>
