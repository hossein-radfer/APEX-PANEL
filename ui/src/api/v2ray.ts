import {
  AssignedXuiPanels,
  AssignedXuiPanelsResponseSchema,
  AssignedXuiPanelSummaries,
  AssignedXuiPanelSummariesResponseSchema,
  AssignResellerXuiPanelsRequest,
  BulkCreateV2RayPackageRequest,
  BulkCreateV2RayPackageResponseSchema,
  BulkCreateV2RayPackageSchema,
  CreateV2RayPackageRequest,
  CreateV2RayPackageSchema,
  ExportV2RayPackagesRequest,
  UpdateV2RayPackageRequest,
  UpdateV2RayPackageShareExpireRequest,
  V2RayAdminSummary,
  V2RayAdminSummaryResponseSchema,
  V2RayLiveUsage,
  V2RayLiveUsageResponseSchema,
  V2RayPackage,
  V2RayPackageResponseSchema,
  V2RayPackageShare,
  V2RayPackageShareDetails,
  V2RayPackageShareDetailsResponseSchema,
  V2RayPackageShareResponseSchema,
  V2RayPackagesResponseSchema,
  V2RaySaleTitle,
  V2RaySaleTitleRequest,
  V2RaySaleTitleResponseSchema,
  V2RaySaleTitlesResponseSchema,
  V2RaySelfSummary,
  V2RaySelfSummaryResponseSchema,
} from '@/schema/v2ray.ts'
import axiosInstance from '@/api/axios-instance.ts'
import { triggerBlobDownload } from '@/lib/download.ts'

export const fetchV2RayPackagesList = async (): Promise<V2RayPackage[]> => {
  const { data } = await axiosInstance.get('/v2ray-package')
  const parsed = V2RayPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchV2RayPackagesByReseller = async (
  resellerId: number
): Promise<V2RayPackage[]> => {
  const { data } = await axiosInstance.get(
    `/v2ray-package/reseller/${resellerId}`
  )
  const parsed = V2RayPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const createV2RayPackage = async (
  pkg: CreateV2RayPackageRequest
): Promise<V2RayPackage> => {
  const validated = CreateV2RayPackageSchema.parse(pkg)
  const { data } = await axiosInstance.post('/v2ray-package', validated)
  const parsed = V2RayPackageResponseSchema.parse(data)
  return parsed.data
}

export const createV2RayPackageForReseller = async ({
  resellerId,
  pkg,
}: {
  resellerId: number
  pkg: CreateV2RayPackageRequest
}): Promise<V2RayPackage> => {
  const validated = CreateV2RayPackageSchema.parse(pkg)
  const { data } = await axiosInstance.post(
    `/v2ray-package/reseller/${resellerId}`,
    validated
  )
  const parsed = V2RayPackageResponseSchema.parse(data)
  return parsed.data
}

export const bulkCreateV2RayPackages = async (
  req: BulkCreateV2RayPackageRequest
): Promise<V2RayPackage[]> => {
  const validated = BulkCreateV2RayPackageSchema.parse(req)
  const { data } = await axiosInstance.post('/v2ray-package/bulk', validated)
  const parsed = BulkCreateV2RayPackageResponseSchema.parse(data)
  return parsed.data.packages || []
}

export const exportV2RayPackages = async (
  req: ExportV2RayPackagesRequest
): Promise<void> => {
  const response = await axiosInstance.post('/v2ray-package/export', req, {
    responseType: 'blob',
  })
  const mimeType =
    req.format === 'txt'
      ? 'text/plain'
      : 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  const blob = new Blob([response.data], { type: mimeType })
  const filename = req.format === 'txt' ? 'v2ray-packages.txt' : 'v2ray-packages.xlsx'
  triggerBlobDownload(blob, filename)
}

export const updateV2RayPackage = async (
  pkg: UpdateV2RayPackageRequest
): Promise<V2RayPackage> => {
  const { data } = await axiosInstance.put(`/v2ray-package/${pkg.id}`, pkg)
  const parsed = V2RayPackageResponseSchema.parse(data)
  return parsed.data
}

export const updateV2RayPackageForReseller = async ({
  resellerId,
  pkg,
}: {
  resellerId: number
  pkg: UpdateV2RayPackageRequest
}): Promise<V2RayPackage> => {
  const { data } = await axiosInstance.put(
    `/v2ray-package/reseller/${resellerId}/${pkg.id}`,
    pkg
  )
  const parsed = V2RayPackageResponseSchema.parse(data)
  return parsed.data
}

export const deleteV2RayPackage = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/v2ray-package/${id}`)
}

export const resetV2RayPackageUsage = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/v2ray-package/${id}/reset-usage`)
}

export const bulkDeleteV2RayPackages = async (
  ids: number[]
): Promise<{ deleted: number[]; failedCount: number }> => {
  const { data } = await axiosInstance.post('/v2ray-package/bulk-delete', { ids })
  return {
    deleted: data?.data?.deleted ?? [],
    failedCount: data?.data?.failed?.length ?? 0,
  }
}

export const deleteV2RayPackageForReseller = async ({
  resellerId,
  packageId,
}: {
  resellerId: number
  packageId: number
}): Promise<void> => {
  await axiosInstance.delete(
    `/v2ray-package/reseller/${resellerId}/${packageId}`
  )
}

export const fetchV2RayPackageShareStatus = async (
  id: number
): Promise<V2RayPackageShare> => {
  const { data } = await axiosInstance.get(`/v2ray-package/${id}/share`)
  const parsed = V2RayPackageShareResponseSchema.parse(data)
  return parsed.data
}

export const updateV2RayPackageShareStatus = async (
  id: number
): Promise<void> => {
  await axiosInstance.patch(`/v2ray-package/${id}/share/status`)
}

export const updateV2RayPackageShareExpire = async (
  pkg: UpdateV2RayPackageShareExpireRequest
): Promise<void> => {
  await axiosInstance.patch(`/v2ray-package/${pkg.id}/share/expire`, pkg)
}

// fetchV2RayLiveUsage calls x-ui LIVE for every one of this package's
// locations (not the periodic sync job's cache) -- used only by the
// on-demand "view live usage" action, never by the ordinary list view.
export const fetchV2RayLiveUsage = async (
  id: number
): Promise<V2RayLiveUsage> => {
  const { data } = await axiosInstance.get(`/v2ray-package/${id}/live-usage`)
  const parsed = V2RayLiveUsageResponseSchema.parse(data)
  return parsed.data
}

export const fetchV2RaySelfSummary = async (): Promise<V2RaySelfSummary> => {
  const { data } = await axiosInstance.get('/v2ray-package/summary/self')
  const parsed = V2RaySelfSummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchV2RayAdminSummary = async (): Promise<V2RayAdminSummary> => {
  const { data } = await axiosInstance.get('/v2ray-package/summary/admin')
  const parsed = V2RayAdminSummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchV2RaySaleTitles = async (): Promise<V2RaySaleTitle[]> => {
  const { data } = await axiosInstance.get('/v2ray-package/sale-title')
  const parsed = V2RaySaleTitlesResponseSchema.parse(data)
  return parsed.data || []
}

export const setV2RaySaleTitle = async (
  request: V2RaySaleTitleRequest
): Promise<V2RaySaleTitle> => {
  const { data } = await axiosInstance.put(
    '/v2ray-package/sale-title',
    request
  )
  const parsed = V2RaySaleTitleResponseSchema.parse(data)
  return parsed.data
}

export const setV2RaySaleTitleForReseller = async ({
  resellerId,
  request,
}: {
  resellerId: number
  request: V2RaySaleTitleRequest
}): Promise<V2RaySaleTitle> => {
  const { data } = await axiosInstance.put(
    `/v2ray-package/reseller/${resellerId}/sale-title`,
    request
  )
  const parsed = V2RaySaleTitleResponseSchema.parse(data)
  return parsed.data
}

// Public, unauthenticated share-page details.
export const fetchV2RayPackageShareDetails = async (
  uuid: string
): Promise<V2RayPackageShareDetails> => {
  const { data } = await axiosInstance.get(`/v2ray-share/${uuid}`)
  const parsed = V2RayPackageShareDetailsResponseSchema.parse(data)
  return parsed.data
}

export const fetchAssignedXuiPanels = async (
  resellerId: number
): Promise<AssignedXuiPanels> => {
  const { data } = await axiosInstance.get(
    `/v2ray-package/reseller/${resellerId}/panels`
  )
  const parsed = AssignedXuiPanelsResponseSchema.parse(data)
  return parsed.data || []
}

// Reseller-safe counterpart of fetchAssignedXuiPanels: id+name instead of
// bare IDs. This is what a reseller session actually calls to render
// V2RayForm's "which server(s)" picker, since GET /xui-panel (the full
// list, with credentials) is admin-only. See the backend route's own doc
// comment (api/http/api.go) for why this endpoint exists.
export const fetchAssignedXuiPanelSummaries = async (
  resellerId: number
): Promise<AssignedXuiPanelSummaries> => {
  const { data } = await axiosInstance.get(
    `/v2ray-package/reseller/${resellerId}/panels/summary`
  )
  const parsed = AssignedXuiPanelSummariesResponseSchema.parse(data)
  return parsed.data || []
}

export const setAssignedXuiPanels = async ({
  resellerId,
  panelIds,
}: {
  resellerId: number
  panelIds: number[]
}): Promise<void> => {
  const body: AssignResellerXuiPanelsRequest = { panel_ids: panelIds }
  await axiosInstance.put(
    `/v2ray-package/reseller/${resellerId}/panels`,
    body
  )
}
