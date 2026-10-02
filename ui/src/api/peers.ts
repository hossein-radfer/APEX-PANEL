import {
  BulkCreatePeerRequest,
  BulkCreatePeerResponseSchema,
  BulkCreatePeerSchema,
  CreatePeerRequest,
  CreatePeerSchema,
  FetchPeerAllowedAddress,
  Peer,
  PeerAllowedAddress,
  PeerAllowedAddressResponseSchema,
  PeerCredentials,
  PeerCredentialsResponseSchema,
  PeerResponseSchema,
  PeerShare,
  PeerShareResponseSchema,
  PeersResponseSchema,
  ResellerActivitySummary,
  ResellerActivitySummaryResponseSchema,
  SelfActivity,
  SelfActivityResponseSchema,
  UpdatePeerRequest,
  UpdatePeerShareExpireRequest,
} from '@/schema/peers.ts'
import {
  SyncPeerPreview,
  SyncPeersRequest,
  SyncPeersRequestSchema,
  SyncPeersResponseSchema,
} from '@/schema/sync.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchPeersList = async (): Promise<Peer[]> => {
  const { data } = await axiosInstance.get('/peer')
  const parsed = PeersResponseSchema.parse(data)
  return parsed.data || []
}

// fetchPeersByReseller powers the admin-only "Reseller Peers" page --
// always scoped to the specific resellerId regardless of who's asking
// (as opposed to fetchPeersList, which scopes to the CALLER's own
// reseller_id when their role is "reseller", or is unscoped for "admin").
export const fetchPeersByReseller = async (resellerId: number): Promise<Peer[]> => {
  const { data } = await axiosInstance.get(`/peer/reseller/${resellerId}`)
  const parsed = PeersResponseSchema.parse(data)
  return parsed.data || []
}

export const createPeerForReseller = async ({
  resellerId,
  peer,
}: {
  resellerId: number
  peer: CreatePeerRequest
}): Promise<Peer> => {
  const validated = CreatePeerSchema.parse(peer)
  const { data } = await axiosInstance.post(`/peer/reseller/${resellerId}`, validated)
  const parsed = PeerResponseSchema.parse(data)
  return parsed.data
}

export const updatePeerForReseller = async ({
  resellerId,
  peer,
}: {
  resellerId: number
  peer: UpdatePeerRequest
}): Promise<Peer> => {
  const { data } = await axiosInstance.put(`/peer/reseller/${resellerId}/${peer.id}`, peer)
  const parsed = PeerResponseSchema.parse(data)
  return parsed.data
}

export const deletePeerForReseller = async ({
  resellerId,
  peerId,
}: {
  resellerId: number
  peerId: number
}): Promise<void> => {
  await axiosInstance.delete(`/peer/reseller/${resellerId}/${peerId}`)
}

export const updatePeerStatusForReseller = async ({
  resellerId,
  peerId,
}: {
  resellerId: number
  peerId: number
}): Promise<void> => {
  await axiosInstance.patch(`/peer/reseller/${resellerId}/${peerId}/status`)
}

export const fetchPeerQRCode = async (id: number): Promise<string> => {
  const response = await axiosInstance.get(`/peer/${id}/qrcode`, {
    responseType: 'blob',
  })

  return URL.createObjectURL(response.data)
}

export const fetchPeerConfig = async (id: number): Promise<Blob> => {
  const response = await axiosInstance.get(`/peer/${id}/config`, {
    responseType: 'blob',
  })

  return response.data
}

export const createPeer = async (peer: CreatePeerRequest): Promise<Peer> => {
  const validated = CreatePeerSchema.parse(peer)
  const { data } = await axiosInstance.post('/peer', validated)
  const parsed = PeerResponseSchema.parse(data)
  return parsed.data
}

export const bulkCreatePeers = async (
  req: BulkCreatePeerRequest
): Promise<Peer[]> => {
  const validated = BulkCreatePeerSchema.parse(req)
  const { data } = await axiosInstance.post('/peer/bulk', validated)
  const parsed = BulkCreatePeerResponseSchema.parse(data)
  return parsed.data.peers || []
}

export const updatePeerStatus = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/peer/${id}/status`)
}

export const updatePeerShareStatus = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/peer/${id}/share/status`)
}

export const updatePeer = async (peer: UpdatePeerRequest): Promise<Peer> => {
  const { data } = await axiosInstance.put(`/peer/${peer.id}`, peer)
  const parsed = PeerResponseSchema.parse(data)
  return parsed.data
}

export const deletePeer = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/peer/${id}`)
}

export const bulkDeletePeers = async (
  ids: number[]
): Promise<{ deleted: number[]; failedCount: number }> => {
  const { data } = await axiosInstance.post('/peer/bulk-delete', { ids })
  return {
    deleted: data?.data?.deleted ?? [],
    failedCount: data?.data?.failed?.length ?? 0,
  }
}

export const fetchPeerAllowedAddress = async (
  iface: FetchPeerAllowedAddress
): Promise<PeerAllowedAddress> => {
  const { data } = await axiosInstance.post(`/peer/allowed-address`, iface)
  const parsed = PeerAllowedAddressResponseSchema.parse(data)
  return parsed.data
}

export const fetchPeerCredentials = async (): Promise<PeerCredentials> => {
  const { data } = await axiosInstance.get('/peer/credentials')
  const parsed = PeerCredentialsResponseSchema.parse(data)
  return parsed.data
}

export const fetchPeerShareStatus = async (id: number): Promise<PeerShare> => {
  const { data } = await axiosInstance.get(`/peer/${id}/share`)
  const parsed = PeerShareResponseSchema.parse(data)
  return parsed.data
}

export const updatePeerShareExpire = async (
  peer: UpdatePeerShareExpireRequest
): Promise<void> => {
  await axiosInstance.patch(`/peer/${peer.id}/share/expire`, peer)
}

export const resetPeerUsage = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/peer/${id}/reset-usage`)
}

export const resetPeerUsages = async (): Promise<void> => {
  await axiosInstance.patch(`/peer/reset-usage`)
}

export const exportTrafficExcel = async (): Promise<void> => {
  const response = await axiosInstance.post(`/peer/traffic/export`, null, {
    responseType: 'blob',
  })

  const blob = new Blob([response.data], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })

  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')

  link.href = url
  link.download = `traffic-report-${new Date().toISOString().split('T')[0]}.xlsx`

  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export const syncPeers = async (): Promise<void> => {
  await axiosInstance.post('/sync/peers')
}

export const fetchSyncPeers = async (
  interfaceName?: string
): Promise<SyncPeerPreview[]> => {
  const { data } = await axiosInstance.get('/sync/peers', {
    params: interfaceName ? { interface: interfaceName } : undefined,
  })
  const parsed = SyncPeersResponseSchema.parse(data)
  return parsed.data || []
}

export const syncSelectedPeers = async (
  payload: SyncPeersRequest
): Promise<void> => {
  const validated = SyncPeersRequestSchema.parse(payload)
  await axiosInstance.post('/sync/peers/selected', validated)
}

export const fetchResellerActivitySummary = async (
  page: number = 1,
  pageSize: number = 20
): Promise<ResellerActivitySummary> => {
  const { data } = await axiosInstance.get('/peer/activity/resellers', {
    params: { page, page_size: pageSize },
  })
  const parsed = ResellerActivitySummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchSelfActivity = async (): Promise<SelfActivity> => {
  const { data } = await axiosInstance.get('/peer/activity/self')
  const parsed = SelfActivityResponseSchema.parse(data)
  return parsed.data
}

