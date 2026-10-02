import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const UserManagerProtocolEnum = z.enum([
  'l2tp',
  'pptp',
  'sstp',
  'openvpn',
  'ikev2',
])

export const USER_MANAGER_PROTOCOLS = UserManagerProtocolEnum.options

// IKEv2 in this list is a display-only tag: selecting it on account
// creation records the customer's intended protocol for their share/
// connection-info page (and surfaces the IKEv2 row's port from
// UserManagerProtocolConfig), but does NOT create or configure any
// IKEv2-specific object on RouterOS -- User Manager itself has no IKEv2
// concept at all (RouterOS IKEv2 is IPsec + mode-config, not a PPP tunnel
// type). The admin must still separately configure IKEv2 access on the
// router (Winbox) for this tag to correspond to anything real. See
// model.ProtocolIKEv2's doc comment on the Go side.
export const ProtocolConfigEnum = z.enum([
  'l2tp',
  'pptp',
  'sstp',
  'openvpn',
  'ikev2',
])
export const PROTOCOL_CONFIG_PROTOCOLS = ProtocolConfigEnum.options

export const UserManagerAccountStatusEnum = z.enum([
  'active',
  'inactive',
  'expired',
  'suspended',
])

export const UserManagerAccountSchema = z.object({
  id: z.number(),
  uuid: z.string(),
  username: z.string(),
  group: z.string(),
  profile: z.string(),
  // One RouterOS account can authenticate over any protocol -- this list
  // controls only which protocols' connection info the panel/share page
  // reveals to this account's customer. See model.UserManagerAccount's Go
  // doc comment for the full rationale.
  protocols: z.array(UserManagerProtocolEnum),
  disabled: z.boolean(),
  comment: z.string().nullable(),
  shared_users: z.number(),
  traffic_limit: z.string().nullable(),
  expire_time: z.string().nullable(),
  total_usage: z.string(),
  status: z.array(UserManagerAccountStatusEnum),
  is_online: z.boolean(),
  is_shared: z.boolean(),
  has_config_file: z.boolean().optional(),
  reseller_id: z.number().nullable().optional(),
  reseller_name: z.string().nullable().optional(),
})

export const UserManagerAccountsSchema = z
  .array(UserManagerAccountSchema)
  .nullable()

export const UserManagerGroupSchema = z.object({
  name: z.string(),
})

export const UserManagerProfileSchema = z.object({
  name: z.string(),
})

export const UserManagerGroupsSchema = z.array(UserManagerGroupSchema)
export const UserManagerProfilesSchema = z.array(UserManagerProfileSchema)

export const UserManagerAccountShareSchema = z.object({
  is_shared: z.boolean(),
  uuid: z.string().nullable(),
  expire_time: z.string().nullable(),
})

export const UserManagerProtocolConfigSchema = z.object({
  id: z.number(),
  protocol: ProtocolConfigEnum,
  port: z.number(),
  server_address: z.string().nullable(),
  certificate_name: z.string().nullable(),
  enabled: z.boolean(),
  notes: z.string().nullable(),
  has_certificate_file: z.boolean().optional(),
  has_client_app_file: z.boolean().optional(),
})

export const UserManagerProtocolConfigsSchema = z.array(
  UserManagerProtocolConfigSchema
)

export const UserManagerShareProtocolInfoSchema = z.object({
  protocol: UserManagerProtocolEnum,
  server_address: z.string(),
  port: z.number(),
  has_certificate_file: z.boolean().optional(),
  has_client_app_file: z.boolean().optional(),
  notes: z.string().nullable().optional(),
})

export const UserManagerAccountShareDetailsSchema = z.object({
  username: z.string(),
  password: z.string(),
  protocols: z.array(UserManagerShareProtocolInfoSchema),
  expire_time: z.string().nullable(),
  has_config_file: z.boolean().optional(),
  traffic_limit: z.string().nullable().optional(),
  download_usage: z.string().optional(),
  upload_usage: z.string().optional(),
  total_usage: z.string().optional(),
  usage_percent: z.string().nullable().optional(),
  is_online: z.boolean().optional(),
})

export const UserManagerAccountResponseSchema = createApiResponseSchema(
  UserManagerAccountSchema
)
export const UserManagerAccountsResponseSchema = createApiResponseSchema(
  UserManagerAccountsSchema
)
export const UserManagerGroupsResponseSchema = createApiResponseSchema(
  UserManagerGroupsSchema
)
export const UserManagerProfilesResponseSchema = createApiResponseSchema(
  UserManagerProfilesSchema
)
export const UserManagerAccountShareResponseSchema = createApiResponseSchema(
  UserManagerAccountShareSchema
)
export const UserManagerProtocolConfigResponseSchema = createApiResponseSchema(
  UserManagerProtocolConfigSchema
)
export const UserManagerProtocolConfigsResponseSchema =
  createApiResponseSchema(UserManagerProtocolConfigsSchema)
export const UserManagerAccountShareDetailsResponseSchema =
  createApiResponseSchema(UserManagerAccountShareDetailsSchema)

export const CreateUserManagerAccountSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  password: z.string().min(1, 'Password is required'),
  group: z.string().min(1, 'Group is required'),
  profile: z.string().min(1, 'Profile is required'),
  protocols: z.array(UserManagerProtocolEnum).min(1, 'Select at least one protocol'),
  shared_users: z.number().optional().nullable(),
  comment: z.string().optional().nullable(),
  traffic_limit: z.string().optional().nullable(),
  expire_time: z.string().optional().nullable(),
})

export const UpdateUserManagerAccountSchema = z.object({
  id: z.number().int().positive(),
  disabled: z.boolean().optional(),
  group: z.string().optional(),
  profile: z.string().optional(),
  protocols: z.array(UserManagerProtocolEnum).min(1).optional(),
  comment: z.string().optional().nullable(),
  shared_users: z.number().optional().nullable(),
  traffic_limit: z.string().optional().nullable(),
  expire_time: z.string().optional().nullable(),
})

export const BulkCreateUserManagerAccountSchema = z.object({
  count: z.number().int().min(1).max(500),
  group: z.string().min(1),
  profile: z.string().min(1),
  protocols: z.array(UserManagerProtocolEnum).min(1),
  shared_users: z.number().optional().nullable(),
  traffic_limit: z.string().optional().nullable(),
  duration_days: z.number().int().positive(),
})

export const BulkCreatedUserManagerAccountSchema = UserManagerAccountSchema.extend({
  password: z.string(),
})

export const BulkCreateUserManagerAccountResultSchema = z.object({
  accounts: z.array(BulkCreatedUserManagerAccountSchema),
})

export const BulkCreateUserManagerAccountResponseSchema = createApiResponseSchema(
  BulkCreateUserManagerAccountResultSchema
)

export type BulkCreateUserManagerAccountRequest = z.infer<
  typeof BulkCreateUserManagerAccountSchema
>
export type BulkCreatedUserManagerAccount = z.infer<
  typeof BulkCreatedUserManagerAccountSchema
>

export const UpdateUserManagerAccountShareExpireSchema = z.object({
  id: z.number().int().positive(),
  expire_time: z.string().optional().nullable(),
})

export const ChangeUserManagerAccountPasswordSchema = z.object({
  password: z.string().min(1, 'Password is required'),
})

export const BulkImportAccountsResultSchema = z.object({
  imported: z.number(),
  skipped: z.number(),
  already_imported: z.number(),
  malformed: z.array(z.string()),
  skipped_usernames: z.array(z.string()),
})

export const UpsertUserManagerProtocolConfigSchema = z.object({
  protocol: ProtocolConfigEnum,
  port: z.number().int().min(1).max(65535),
  server_address: z.string().optional().nullable(),
  certificate_name: z.string().optional().nullable(),
  enabled: z.boolean(),
  notes: z.string().optional().nullable(),
})

export const UserManagerSelfSummarySchema = z.object({
  online_accounts: z.number(),
  total_accounts: z.number(),
  quota_bytes: z.number().nullable(),
  used_bytes: z.number(),
  remaining_bytes: z.number().nullable(),
  max_accounts: z.number().nullable(),
})

export const UserManagerSelfSummaryResponseSchema = createApiResponseSchema(
  UserManagerSelfSummarySchema
)

export const BulkImportAccountsResponseSchema = createApiResponseSchema(
  BulkImportAccountsResultSchema
)

export type UserManagerProtocol = z.infer<typeof UserManagerProtocolEnum>
// Includes "ikev2" in addition to UserManagerProtocol's four PPP protocols
// -- used anywhere a protocol-config row, certificate, or client-app file
// is addressed (those support IKEv2 as a config-only entry), never for
// account creation itself.
export type ProtocolConfigProtocol = z.infer<typeof ProtocolConfigEnum>
export type UserManagerAccountStatus = z.infer<
  typeof UserManagerAccountStatusEnum
>
export type UserManagerAccount = z.infer<typeof UserManagerAccountSchema>
export type UserManagerGroup = z.infer<typeof UserManagerGroupSchema>
export type UserManagerProfile = z.infer<typeof UserManagerProfileSchema>
export type UserManagerAccountShare = z.infer<
  typeof UserManagerAccountShareSchema
>
export type UserManagerProtocolConfig = z.infer<
  typeof UserManagerProtocolConfigSchema
>
export type UserManagerAccountShareDetails = z.infer<
  typeof UserManagerAccountShareDetailsSchema
>
export type UserManagerShareProtocolInfo = z.infer<
  typeof UserManagerShareProtocolInfoSchema
>
export type CreateUserManagerAccountRequest = z.infer<
  typeof CreateUserManagerAccountSchema
>
export type UpdateUserManagerAccountRequest = z.infer<
  typeof UpdateUserManagerAccountSchema
>
export type UpdateUserManagerAccountShareExpireRequest = z.infer<
  typeof UpdateUserManagerAccountShareExpireSchema
>
export type ChangeUserManagerAccountPasswordRequest = z.infer<
  typeof ChangeUserManagerAccountPasswordSchema
>
export type BulkImportAccountsResult = z.infer<
  typeof BulkImportAccountsResultSchema
>
export type UpsertUserManagerProtocolConfigRequest = z.infer<
  typeof UpsertUserManagerProtocolConfigSchema
>
export type UserManagerSelfSummary = z.infer<
  typeof UserManagerSelfSummarySchema
>
