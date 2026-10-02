import { createFileRoute } from '@tanstack/react-router'
import SettingsLicense from '@/features/settings/license'

export const Route = createFileRoute('/_authenticated/settings/license')({
  component: SettingsLicense,
})
