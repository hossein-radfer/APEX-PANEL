import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const BotSettingsSchema = z.object({
  bot_token_set: z.boolean(),
  bot_token_masked: z.string(),
  admin_chat_id: z.string(),
  enabled: z.boolean(),
  notify_login_alerts: z.boolean(),
  notify_purchase_receipts: z.boolean(),
  notify_live_log: z.boolean(),
  notify_quota_warnings: z.boolean(),
  notify_critical_alerts: z.boolean(),
  notify_account_status: z.boolean(),
  otp_enabled: z.boolean(),
  auto_backup_enabled: z.boolean(),
  auto_backup_hour: z.number(),
  auto_backup_minute: z.number(),
  // Read-only: the daily report schedule is set exclusively via the bot's
  // /setreport command, not this settings page.
  auto_report_enabled: z.boolean(),
  auto_report_hour: z.number(),
  auto_report_minute: z.number(),
  socks5_enabled: z.boolean(),
  socks5_address: z.string(),
  socks5_username: z.string(),
  socks5_password_set: z.boolean(),
  faoxima_domain: z.string(),
})

export const BotSettingsResponseSchema = createApiResponseSchema(BotSettingsSchema)

export type BotSettings = z.infer<typeof BotSettingsSchema>

// bot_token is deliberately omitted here on purpose when unset -- the
// update form only ever sends it when the admin actually typed a new
// token, never the masked placeholder the GET response returns.
export const ExtraAdminChatIDSchema = z.object({
  id: z.number(),
  chat_id: z.string(),
  label: z.string().nullable().optional(),
})
export const ExtraAdminChatIDsResponseSchema = createApiResponseSchema(
  z.array(ExtraAdminChatIDSchema)
)
export type ExtraAdminChatID = z.infer<typeof ExtraAdminChatIDSchema>

export interface AddExtraAdminChatIDRequest {
  chat_id: string
  label?: string | null
}

export interface UpdateBotSettingsRequest {
  bot_token?: string
  admin_chat_id?: string
  enabled?: boolean
  notify_login_alerts?: boolean
  notify_purchase_receipts?: boolean
  notify_live_log?: boolean
  notify_quota_warnings?: boolean
  notify_critical_alerts?: boolean
  notify_account_status?: boolean
  otp_enabled?: boolean
  socks5_enabled?: boolean
  socks5_address?: string
  socks5_username?: string
  socks5_password?: string
  faoxima_domain?: string
}

// TestSocks5Response mirrors api/http/schema.TestSocks5Response -- item 5's
// live connection-status + ping display on the settings page.
export const TestSocks5ResponseSchema = z.object({
  connected: z.boolean(),
  ping_ms: z.number().optional(),
  error: z.string().optional(),
})
export const TestSocks5ApiResponseSchema = createApiResponseSchema(
  TestSocks5ResponseSchema
)
export type TestSocks5Result = z.infer<typeof TestSocks5ResponseSchema>

export interface TestSocks5Request {
  address: string
  username?: string
  password?: string
  password_unchanged?: boolean
}
