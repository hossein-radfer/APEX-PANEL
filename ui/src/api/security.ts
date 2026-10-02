import {
  EtherTrafficResponseSchema,
  EtherTrafficRow,
  GeoIPMode,
  GeoIPModeResponseSchema,
  GeoIPStatus,
  GeoIPStatusResponseSchema,
  RetentionCleanupRequest,
  RetentionCleanupResponseSchema,
  SecurityHistory,
  SecurityHistoryResponseSchema,
  SecurityIdentitiesResponseSchema,
  SecurityIdentity,
  SecurityThreat,
  SecurityThreatsResponseSchema,
} from '@/schema/security.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchGeoIPStatus = async (): Promise<GeoIPStatus> => {
  const { data } = await axiosInstance.get('/security/geoip/status')
  return GeoIPStatusResponseSchema.parse(data).data
}

export const fetchGeoIPMode = async (): Promise<GeoIPMode> => {
  const { data } = await axiosInstance.get('/security/geoip/mode')
  return GeoIPModeResponseSchema.parse(data).data
}

export const setGeoIPMode = async (onlineEnabled: boolean): Promise<void> => {
  await axiosInstance.put('/security/geoip/mode', { online_enabled: onlineEnabled })
}

const uploadGeoIPFile = async (
  path: string,
  file: File,
  onProgress?: (percent: number) => void
): Promise<void> => {
  const formData = new FormData()
  formData.append('file', file)
  await axiosInstance.post(path, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    onUploadProgress: (event) => {
      if (!onProgress || !event.total) return
      onProgress(Math.round((event.loaded / event.total) * 100))
    },
  })
}

export const uploadGeoIPCity = (file: File, onProgress?: (percent: number) => void) =>
  uploadGeoIPFile('/security/geoip/city', file, onProgress)
export const uploadGeoIPASN = (file: File, onProgress?: (percent: number) => void) =>
  uploadGeoIPFile('/security/geoip/asn', file, onProgress)

export const deleteGeoIPCity = async (): Promise<void> => {
  await axiosInstance.delete('/security/geoip/city')
}
export const deleteGeoIPASN = async (): Promise<void> => {
  await axiosInstance.delete('/security/geoip/asn')
}
export const clearGeoIPCache = async (): Promise<void> => {
  await axiosInstance.delete('/security/geoip/cache')
}

export const fetchSecurityIdentities = async (
  search?: string,
  onlineOnly?: boolean
): Promise<SecurityIdentity[]> => {
  const { data } = await axiosInstance.get('/security/identities', {
    params: {
      search: search || undefined,
      online_only: onlineOnly === undefined ? undefined : String(onlineOnly),
    },
  })
  return SecurityIdentitiesResponseSchema.parse(data).data.identities
}

export const fetchSecurityIdentityHistory = async (
  protocol: string,
  identity: string
): Promise<SecurityHistory> => {
  const { data } = await axiosInstance.get(
    `/security/identities/${encodeURIComponent(protocol)}/${encodeURIComponent(identity)}/history`
  )
  return SecurityHistoryResponseSchema.parse(data).data
}

export const fetchSecurityThreats = async (): Promise<SecurityThreat[]> => {
  const { data } = await axiosInstance.get('/security/threats')
  return SecurityThreatsResponseSchema.parse(data).data.threats
}

export const clearConnectionHistory = async (): Promise<void> => {
  await axiosInstance.delete('/security/connection-log')
}

export const fetchEtherTraffic = async (): Promise<EtherTrafficRow[]> => {
  const { data } = await axiosInstance.get('/security/ether-traffic')
  return EtherTrafficResponseSchema.parse(data).data.flows
}

export const clearEtherTraffic = async (): Promise<void> => {
  await axiosInstance.delete('/security/ether-traffic')
}

export const runRetentionCleanup = async (
  req: RetentionCleanupRequest
): Promise<number> => {
  const { data } = await axiosInstance.post('/security/retention/cleanup', req)
  return RetentionCleanupResponseSchema.parse(data).data.deleted_count
}
