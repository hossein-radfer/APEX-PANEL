import {
  Application,
  AppMaintenanceMode,
  AppMaintenanceModeResponseSchema,
  ApplicationOpenVpnTemplateStatus,
  ApplicationOpenVpnTemplateStatusResponseSchema,
  ApplicationResourceLocationsResponseSchema,
  ApplicationResponseSchema,
  ApplicationsResponseSchema,
  AppVersion,
  AppVersionResponseSchema,
  AppVersionsResponseSchema,
  CreateApplicationRequest,
  CreateApplicationSchema,
  PublishAppVersionRequest,
  SetApplicationResourceLocationRequest,
  SetAppMaintenanceModeRequest,
  UpdateApplicationRequest,
} from '@/schema/application.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchApplicationsList = async (): Promise<Application[]> => {
  const { data } = await axiosInstance.get('/application')
  const parsed = ApplicationsResponseSchema.parse(data)
  return parsed.data.applications || []
}

export const fetchApplicationsByReseller = async (
  resellerId: number
): Promise<Application[]> => {
  const { data } = await axiosInstance.get(
    `/application/reseller/${resellerId}`
  )
  const parsed = ApplicationsResponseSchema.parse(data)
  return parsed.data.applications || []
}

export const createApplication = async (
  req: CreateApplicationRequest
): Promise<Application> => {
  const validated = CreateApplicationSchema.parse(req)
  const { data } = await axiosInstance.post('/application', validated)
  const parsed = ApplicationResponseSchema.parse(data)
  return parsed.data
}

export const createApplicationForReseller = async ({
  resellerId,
  app,
}: {
  resellerId: number
  app: CreateApplicationRequest
}): Promise<Application> => {
  const validated = CreateApplicationSchema.parse(app)
  const { data } = await axiosInstance.post(
    `/application/reseller/${resellerId}`,
    validated
  )
  const parsed = ApplicationResponseSchema.parse(data)
  return parsed.data
}

export const updateApplication = async (
  req: UpdateApplicationRequest
): Promise<Application> => {
  const { id, ...body } = req
  const { data } = await axiosInstance.put(`/application/${id}`, body)
  const parsed = ApplicationResponseSchema.parse(data)
  return parsed.data
}

// Mirrors createApplicationForReseller's own admin-on-behalf-of-reseller
// pattern -- see UpdateApplicationForReseller's own Go doc comment for
// the reported gap this fixes (the generic updateApplication route above
// never enforced reseller-ownership checks when called by an admin).
export const updateApplicationForReseller = async ({
  resellerId,
  app,
}: {
  resellerId: number
  app: UpdateApplicationRequest
}): Promise<Application> => {
  const { id, ...body } = app
  const { data } = await axiosInstance.put(
    `/application/reseller/${resellerId}/${id}`,
    body
  )
  const parsed = ApplicationResponseSchema.parse(data)
  return parsed.data
}

export const deleteApplication = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/application/${id}`)
}

export const fetchApplicationResourceLocations = async () => {
  const { data } = await axiosInstance.get('/application/resource-location')
  const parsed = ApplicationResourceLocationsResponseSchema.parse(data)
  return parsed.data.locations || []
}

export const setApplicationResourceLocation = async (
  req: SetApplicationResourceLocationRequest
): Promise<void> => {
  await axiosInstance.put('/application/resource-location', req)
}

// --- Admin-only global OpenVPN template ---

export const fetchApplicationOpenVpnTemplateStatus =
  async (): Promise<ApplicationOpenVpnTemplateStatus> => {
    const { data } = await axiosInstance.get('/application/openvpn-template')
    const parsed = ApplicationOpenVpnTemplateStatusResponseSchema.parse(data)
    return parsed.data
  }

export const uploadApplicationOpenVpnTemplate = async (
  file: File
): Promise<void> => {
  const formData = new FormData()
  formData.append('template', file)
  await axiosInstance.post('/application/openvpn-template', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

// --- Admin-only mobile app version management ("مدیریت اپلیکیشن") ------

export const fetchAppVersions = async (): Promise<AppVersion[]> => {
  const { data } = await axiosInstance.get('/application-version')
  const parsed = AppVersionsResponseSchema.parse(data)
  return parsed.data.versions || []
}

export const publishAppVersion = async (
  req: PublishAppVersionRequest
): Promise<AppVersion> => {
  const { data } = await axiosInstance.post('/application-version', req)
  const parsed = AppVersionResponseSchema.parse(data)
  return parsed.data
}

export const deleteAppVersion = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/application-version/${id}`)
}

export const fetchAppMaintenanceMode = async (): Promise<AppMaintenanceMode> => {
  const { data } = await axiosInstance.get(
    '/application-version/maintenance-mode'
  )
  const parsed = AppMaintenanceModeResponseSchema.parse(data)
  return parsed.data
}

export const setAppMaintenanceMode = async (
  req: SetAppMaintenanceModeRequest
): Promise<AppMaintenanceMode> => {
  const { data } = await axiosInstance.put(
    '/application-version/maintenance-mode',
    req
  )
  const parsed = AppMaintenanceModeResponseSchema.parse(data)
  return parsed.data
}
