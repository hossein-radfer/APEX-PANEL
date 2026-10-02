import { createFileRoute } from '@tanstack/react-router'
import SettingsResellerBot from '@/features/settings/reseller-bot'

export const Route = createFileRoute('/_authenticated/settings/reseller-bot')({
  component: SettingsResellerBot,
})
