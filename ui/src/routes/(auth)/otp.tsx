import { z } from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import Otp from '@/features/auth/otp'

export const Route = createFileRoute('/(auth)/otp')({
  component: Otp,

  validateSearch: z.object({
    otp_token: z.string().min(1),
  }),
})
