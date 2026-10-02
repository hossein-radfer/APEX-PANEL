import {
  CreateDNSPanelRequest,
  CreateDNSPanelSchema,
  DNSPanel,
  DNSPanelResponseSchema,
  DNSPanelsResponseSchema,
  DNSTestConnectionResult,
  DNSTestConnectionResultResponseSchema,
  TestDNSPanelRequest,
  TestDNSPanelSchema,
  UpdateDNSPanelRequest,
} from '@/schema/dns-panel.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchDNSPanelsList = async (): Promise<DNSPanel[]> => {
  const { data } = await axiosInstance.get('/dns-panel')
  const parsed = DNSPanelsResponseSchema.parse(data)
  return parsed.data || []
}

export const createDNSPanel = async (
  panel: CreateDNSPanelRequest
): Promise<DNSPanel> => {
  const validated = CreateDNSPanelSchema.parse(panel)
  const { data } = await axiosInstance.post('/dns-panel', validated)
  const parsed = DNSPanelResponseSchema.parse(data)
  return parsed.data
}

export const updateDNSPanel = async (
  panel: UpdateDNSPanelRequest
): Promise<DNSPanel> => {
  const { data } = await axiosInstance.put(`/dns-panel/${panel.id}`, panel)
  const parsed = DNSPanelResponseSchema.parse(data)
  return parsed.data
}

export const deleteDNSPanel = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/dns-panel/${id}`)
}

// Tests an already-saved panel's stored credentials. Admin only server-side
// (GET/POST /dns-panel/* is an admin-only route group) -- callers reachable
// by a reseller MUST use testSavedDNSPanelForReseller below instead, or
// this 403s.
export const testSavedDNSPanel = async (
  id: number
): Promise<DNSTestConnectionResult> => {
  const { data } = await axiosInstance.post(`/dns-panel/${id}/test`)
  const parsed = DNSTestConnectionResultResponseSchema.parse(data)
  return parsed.data
}

// Reseller-safe counterpart to testSavedDNSPanel -- a confirmed, reported
// bug: DNSAccountForm's own "بارگذاری پلن‌ها" (load plans) button called
// testSavedDNSPanel unconditionally for every caller, so a reseller
// creating their own DNS account always got a 403 there (the admin-only
// endpoint above). This hits the reseller-scoped test route instead, which
// verifies the panel is actually one this reseller was assigned before
// probing it.
export const testSavedDNSPanelForReseller = async (
  resellerId: number,
  id: number
): Promise<DNSTestConnectionResult> => {
  const { data } = await axiosInstance.post(
    `/dns-account/reseller/${resellerId}/panels/${id}/test`
  )
  const parsed = DNSTestConnectionResultResponseSchema.parse(data)
  return parsed.data
}

// Tests in-progress form values before the panel has been saved -- used by
// the create dialog to show the returned template list read-only.
export const testUnsavedDNSPanel = async (
  panel: TestDNSPanelRequest
): Promise<DNSTestConnectionResult> => {
  const validated = TestDNSPanelSchema.parse(panel)
  const { data } = await axiosInstance.post('/dns-panel/test', validated)
  const parsed = DNSTestConnectionResultResponseSchema.parse(data)
  return parsed.data
}
