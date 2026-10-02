import { createFileRoute } from '@tanstack/react-router'
import LicenseActivation from '@/features/auth/license-activation'

export const Route = createFileRoute('/(auth)/license-activation')({
  component: LicenseActivation,
})
