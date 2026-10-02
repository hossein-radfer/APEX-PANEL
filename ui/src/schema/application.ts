import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const ApplicationResourceSchema = z.object({
  resource_id: z.number(),
  resource_name: z.string(),
  profile_name: z.string().nullable().optional(),
  label: z.string().nullable().optional(),
  enabled: z.boolean(),
})

export const ApplicationSchema = z.object({
  id: z.number(),
  reseller_id: z.number().nullable().optional(),
  name: z.string(),
  app_username: z.string(),
  app_password: z.string(),
  total_volume_bytes: z.number(),
  used_bytes: z.number(),
  duration_days: z.number(),
  start_at: z.string().nullable().optional(),
  expire_at: z.string().nullable().optional(),
  max_online_users: z.number(),
  status: z.enum(['active', 'suspended', 'expired']),
  disabled: z.boolean(),
  download_speed_limit_mbps: z.number().nullable().optional(),
  upload_speed_limit_mbps: z.number().nullable().optional(),
  wireguard_peers: z.array(ApplicationResourceSchema),
  user_manager_accounts: z.array(ApplicationResourceSchema),
  v2ray_packages: z.array(ApplicationResourceSchema),
  // Only ever populated on the CREATE response -- see the backend's own
  // ApplicationResponse.ProvisioningErrors doc comment for why one
  // failed resource no longer silently blocks the rest.
  provisioning_errors: z.array(z.string()).optional(),
})

export const ApplicationsSchema = z.object({
  applications: z.array(ApplicationSchema),
})

export const ApplicationResponseSchema = createApiResponseSchema(ApplicationSchema)
export const ApplicationsResponseSchema = createApiResponseSchema(ApplicationsSchema)

export type Application = z.infer<typeof ApplicationSchema>
export type ApplicationResource = z.infer<typeof ApplicationResourceSchema>

export const ApplicationGroupProfileSchema = z.object({
  group_name: z.string().min(1),
  profile_name: z.string().min(1),
})

export const CreateApplicationSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  interface_ids: z.array(z.number()).optional(),
  user_manager_groups: z.array(ApplicationGroupProfileSchema).optional(),
  xui_panel_ids: z.array(z.number()).optional(),
  // application_plan_id, when set, lets the backend fill in
  // total_volume_bytes/duration_days/max_online_users from the plan -- see
  // that field's own doc comment on http.schema.CreateApplicationRequest.
  // These three are `.min(0)` rather than `.positive()` here specifically
  // to allow that plan-only submission (a genuine 0 is still rejected
  // server-side via required_without=ApplicationPlanID when no plan is
  // set -- ApplicationForm's own pre-submit checks already enforce the
  // "positive when no plan chosen" rule before this schema ever runs).
  application_plan_id: z.number().optional().nullable(),
  total_volume_bytes: z.number().min(0, 'Total volume must not be negative'),
  duration_days: z.number().int().min(0, 'Duration must not be negative'),
  max_online_users: z.number().int().min(0, 'Must not be negative'),
  app_username: z.string().optional().nullable(),
  app_password: z.string().optional().nullable(),
  // Client-side (mobile-app-enforced) throughput cap, applied uniformly
  // across whichever protocol the user connects with -- optional, empty
  // meaning unlimited. See the backend's own model.Application doc
  // comment for why this is enforced by the app, not a MikroTik queue.
  download_speed_limit_mbps: z.number().int().positive().optional().nullable(),
  upload_speed_limit_mbps: z.number().int().positive().optional().nullable(),
})

export const UpdateApplicationSchema = z.object({
  name: z.string().optional(),
  total_volume_bytes: z.number().positive().optional(),
  duration_days: z.number().int().positive().optional(),
  max_online_users: z.number().int().positive().optional(),
  disabled: z.boolean().optional(),
  download_speed_limit_mbps: z.number().int().positive().optional().nullable(),
  upload_speed_limit_mbps: z.number().int().positive().optional().nullable(),
  // Omitting one of these three fields leaves that resource type
  // untouched; sending it (even as an empty array) REPLACES this
  // Application's full set of that resource type. See the backend's own
  // UpdateApplicationRequest doc comment for the reported gap this
  // fixes (there was previously no way to edit an existing
  // Application's protocols/interfaces/locations at all).
  interface_ids: z.array(z.number()).optional(),
  user_manager_groups: z.array(ApplicationGroupProfileSchema).optional(),
  xui_panel_ids: z.array(z.number()).optional(),
})

export type CreateApplicationRequest = z.infer<typeof CreateApplicationSchema>
export type UpdateApplicationRequest = z.infer<typeof UpdateApplicationSchema> & {
  id: number
}

// --- Admin-only resource location labels ---

export const ApplicationResourceLocationSchema = z.object({
  resource_type: z.enum(['wireguard_interface', 'user_manager_group', 'xui_panel']),
  resource_key: z.string(),
  label: z.string(),
})

export const ApplicationResourceLocationsSchema = z.object({
  locations: z.array(ApplicationResourceLocationSchema),
})

export const ApplicationResourceLocationsResponseSchema = createApiResponseSchema(
  ApplicationResourceLocationsSchema
)

export type ApplicationResourceLocation = z.infer<
  typeof ApplicationResourceLocationSchema
>

export interface SetApplicationResourceLocationRequest {
  resource_type: 'wireguard_interface' | 'user_manager_group' | 'xui_panel'
  resource_key: string
  label: string
}

// --- Admin-only global OpenVPN template ---

export const ApplicationOpenVpnTemplateStatusSchema = z.object({
  exists: z.boolean(),
  uploaded_at: z.string().nullable().optional(),
})

export const ApplicationOpenVpnTemplateStatusResponseSchema =
  createApiResponseSchema(ApplicationOpenVpnTemplateStatusSchema)

export type ApplicationOpenVpnTemplateStatus = z.infer<
  typeof ApplicationOpenVpnTemplateStatusSchema
>

// --- Admin-only mobile app version management ("مدیریت اپلیکیشن") ------

export const AppVersionSchema = z.object({
  id: z.number(),
  version_code: z.number(),
  version_name: z.string(),
  release_notes: z.string().nullable().optional(),
  download_url: z.string(),
  is_mandatory: z.boolean(),
  published_at: z.string(),
})

export const AppVersionsSchema = z.object({
  versions: z.array(AppVersionSchema),
})

export const AppVersionResponseSchema = createApiResponseSchema(AppVersionSchema)
export const AppVersionsResponseSchema = createApiResponseSchema(AppVersionsSchema)

export type AppVersion = z.infer<typeof AppVersionSchema>

export const PublishAppVersionSchema = z.object({
  version_code: z.number().int().positive('کد نسخه باید عدد مثبت باشد'),
  version_name: z.string().min(1, 'نام نسخه الزامی است'),
  release_notes: z.string().optional().nullable(),
  download_url: z.string().url('لینک دانلود معتبر نیست'),
  is_mandatory: z.boolean(),
})

export type PublishAppVersionRequest = z.infer<typeof PublishAppVersionSchema>

export const AppMaintenanceModeSchema = z.object({
  enabled: z.boolean(),
  message: z.string().nullable().optional(),
})

export const AppMaintenanceModeResponseSchema = createApiResponseSchema(
  AppMaintenanceModeSchema
)

export type AppMaintenanceMode = z.infer<typeof AppMaintenanceModeSchema>

export interface SetAppMaintenanceModeRequest {
  enabled: boolean
  message?: string | null
}
