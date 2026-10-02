import {
  AssignedDNSPanels,
  AssignedDNSPanelsResponseSchema,
  DNSPanelSummaries,
  DNSPanelSummariesResponseSchema,
} from '@/schema/dns-panel.ts'
import {
  AssignResellerDNSPanelsRequest,
  CreateDNSAccountRequest,
  CreateDNSAccountSchema,
  DNSAccount,
  DNSAccountResponseSchema,
  DNSAccountShareDetails,
  DNSAccountShareDetailsResponseSchema,
  DNSAccountShareStatus,
  DNSAccountShareStatusResponseSchema,
  DNSAccountsResponseSchema,
  DNSAdminSummary,
  DNSAdminSummaryResponseSchema,
  DNSSelfSummary,
  DNSSelfSummaryResponseSchema,
  RegisterDNSIPResult,
  RegisterDNSIPResultResponseSchema,
  UpdateDNSAccountRequest,
  UpdateDNSAccountShareExpireRequest,
} from '@/schema/dns-account.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchDNSAccountsList = async (): Promise<DNSAccount[]> => {
  const { data } = await axiosInstance.get('/dns-account')
  const parsed = DNSAccountsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchDNSAccountsByReseller = async (
  resellerId: number
): Promise<DNSAccount[]> => {
  const { data } = await axiosInstance.get(
    `/dns-account/reseller/${resellerId}`
  )
  const parsed = DNSAccountsResponseSchema.parse(data)
  return parsed.data || []
}

export const createDNSAccount = async (
  account: CreateDNSAccountRequest
): Promise<DNSAccount> => {
  const validated = CreateDNSAccountSchema.parse(account)
  const { data } = await axiosInstance.post('/dns-account', validated)
  const parsed = DNSAccountResponseSchema.parse(data)
  return parsed.data
}

export const createDNSAccountForReseller = async ({
  resellerId,
  account,
}: {
  resellerId: number
  account: CreateDNSAccountRequest
}): Promise<DNSAccount> => {
  const validated = CreateDNSAccountSchema.parse(account)
  const { data } = await axiosInstance.post(
    `/dns-account/reseller/${resellerId}`,
    validated
  )
  const parsed = DNSAccountResponseSchema.parse(data)
  return parsed.data
}

export const updateDNSAccount = async (
  account: UpdateDNSAccountRequest
): Promise<DNSAccount> => {
  const { data } = await axiosInstance.put(
    `/dns-account/${account.id}`,
    account
  )
  const parsed = DNSAccountResponseSchema.parse(data)
  return parsed.data
}

export const updateDNSAccountForReseller = async ({
  resellerId,
  account,
}: {
  resellerId: number
  account: UpdateDNSAccountRequest
}): Promise<DNSAccount> => {
  const { data } = await axiosInstance.put(
    `/dns-account/reseller/${resellerId}/${account.id}`,
    account
  )
  const parsed = DNSAccountResponseSchema.parse(data)
  return parsed.data
}

export const deleteDNSAccount = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/dns-account/${id}`)
}

export const deleteDNSAccountForReseller = async ({
  resellerId,
  accountId,
}: {
  resellerId: number
  accountId: number
}): Promise<void> => {
  await axiosInstance.delete(`/dns-account/reseller/${resellerId}/${accountId}`)
}

export const resetDNSAccountUsage = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/dns-account/${id}/reset-usage`)
}

export const bulkDeleteDNSAccounts = async (
  ids: number[]
): Promise<{ deleted: number[]; failedCount: number }> => {
  const { data } = await axiosInstance.post('/dns-account/bulk-delete', {
    ids,
  })
  return {
    deleted: data?.data?.deleted ?? [],
    failedCount: data?.data?.failed?.length ?? 0,
  }
}

export const fetchDNSAccountShareStatus = async (
  id: number
): Promise<DNSAccountShareStatus> => {
  const { data } = await axiosInstance.get(`/dns-account/${id}/share`)
  const parsed = DNSAccountShareStatusResponseSchema.parse(data)
  return parsed.data
}

export const updateDNSAccountShareStatus = async (
  id: number
): Promise<void> => {
  await axiosInstance.patch(`/dns-account/${id}/share/status`)
}

export const updateDNSAccountShareExpire = async (
  account: UpdateDNSAccountShareExpireRequest
): Promise<void> => {
  await axiosInstance.patch(`/dns-account/${account.id}/share/expire`, account)
}

export const fetchDNSSelfSummary = async (): Promise<DNSSelfSummary> => {
  const { data } = await axiosInstance.get('/dns-account/summary/self')
  const parsed = DNSSelfSummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchDNSAdminSummary = async (): Promise<DNSAdminSummary> => {
  const { data } = await axiosInstance.get('/dns-account/summary/admin')
  const parsed = DNSAdminSummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchAssignedDNSPanels = async (
  resellerId: number
): Promise<AssignedDNSPanels> => {
  const { data } = await axiosInstance.get(
    `/dns-account/reseller/${resellerId}/panels`
  )
  const parsed = AssignedDNSPanelsResponseSchema.parse(data)
  return parsed.data || []
}

// Reseller-safe counterpart of fetchAssignedDNSPanels: id+name instead of
// bare IDs. This is what a reseller session calls to render DNSAccountForm's
// "which DNS panel" picker, since GET /dns-panel (the full list, with
// credentials) is admin-only.
export const fetchAssignedDNSPanelSummaries = async (
  resellerId: number
): Promise<DNSPanelSummaries> => {
  const { data } = await axiosInstance.get(
    `/dns-account/reseller/${resellerId}/panels/summary`
  )
  const parsed = DNSPanelSummariesResponseSchema.parse(data)
  return parsed.data || []
}

export const setAssignedDNSPanels = async ({
  resellerId,
  panelIds,
}: {
  resellerId: number
  panelIds: number[]
}): Promise<void> => {
  const body: AssignResellerDNSPanelsRequest = { panel_ids: panelIds }
  await axiosInstance.put(`/dns-account/reseller/${resellerId}/panels`, body)
}

// --- Public, unauthenticated share/status page ------------------------------
// Reached by UUID alone -- no JWT required server-side. Uses the same
// axiosInstance as every other call (it only ever attaches an
// Authorization header when an access_token cookie actually exists, so an
// anonymous visitor's request goes out with no auth header at all), same
// convention as fetchV2RayPackageShareDetails in api/v2ray.ts.

export const fetchDNSAccountShareDetails = async (
  uuid: string
): Promise<DNSAccountShareDetails> => {
  const { data } = await axiosInstance.get(`/dns-share/${uuid}`)
  const parsed = DNSAccountShareDetailsResponseSchema.parse(data)
  return parsed.data
}

// Takes no body/parameters at all -- the backend captures the caller's own
// real IP server-side for security, so there is nothing for the client to
// send here.
export const registerDNSIP = async (
  uuid: string
): Promise<RegisterDNSIPResult> => {
  const { data } = await axiosInstance.post(`/dns-share/${uuid}/register-ip`)
  const parsed = RegisterDNSIPResultResponseSchema.parse(data)
  return parsed.data
}
