import {
  AccountingCost,
  AccountingCostsResponseSchema,
  AccountingPartner,
  AccountingPartnersResponseSchema,
  AccountingPayment,
  AccountingPaymentsResponseSchema,
  AccountingSummary,
  AccountingSummaryResponseSchema,
  CreateCostRequest,
  CreatePartnerRequest,
  CreatePaymentRequest,
  LocationProfitability,
  LocationProfitabilityResponseSchema,
  SellableResource,
  SellableResourcesResponseSchema,
  UserProfitability,
  UserProfitabilityResponseSchema,
} from '@/schema/accounting.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchPartners = async (): Promise<AccountingPartner[]> => {
  const { data } = await axiosInstance.get('/accounting/partners')
  const parsed = AccountingPartnersResponseSchema.parse(data)
  return parsed.data || []
}

export const createPartner = async (
  req: CreatePartnerRequest
): Promise<AccountingPartner> => {
  const { data } = await axiosInstance.post('/accounting/partners', req)
  return data.data
}

export const updatePartner = async ({
  id,
  ...req
}: CreatePartnerRequest & { id: number }): Promise<AccountingPartner> => {
  const { data } = await axiosInstance.put(`/accounting/partners/${id}`, req)
  return data.data
}

export const deletePartner = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/accounting/partners/${id}`)
}

export const fetchCosts = async (): Promise<AccountingCost[]> => {
  const { data } = await axiosInstance.get('/accounting/costs')
  const parsed = AccountingCostsResponseSchema.parse(data)
  return parsed.data || []
}

export const createCost = async (
  req: CreateCostRequest
): Promise<AccountingCost> => {
  const { data } = await axiosInstance.post('/accounting/costs', req)
  return data.data
}

export const updateCost = async ({
  id,
  ...req
}: CreateCostRequest & { id: number }): Promise<AccountingCost> => {
  const { data } = await axiosInstance.put(`/accounting/costs/${id}`, req)
  return data.data
}

export const deleteCost = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/accounting/costs/${id}`)
}

export const advanceRecurringCost = async (id: number): Promise<void> => {
  await axiosInstance.post(`/accounting/costs/${id}/advance`)
}

export const fetchPayments = async (): Promise<AccountingPayment[]> => {
  const { data } = await axiosInstance.get('/accounting/payments')
  const parsed = AccountingPaymentsResponseSchema.parse(data)
  return parsed.data || []
}

export const createPayment = async (
  req: CreatePaymentRequest
): Promise<AccountingPayment> => {
  const { data } = await axiosInstance.post('/accounting/payments', req)
  return data.data
}

export const updatePayment = async ({
  id,
  ...req
}: CreatePaymentRequest & { id: number }): Promise<AccountingPayment> => {
  const { data } = await axiosInstance.put(`/accounting/payments/${id}`, req)
  return data.data
}

export const deletePayment = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/accounting/payments/${id}`)
}

export const uploadReceipt = async ({
  id,
  file,
}: {
  id: number
  file: File
}): Promise<void> => {
  const formData = new FormData()
  formData.append('receipt', file)
  await axiosInstance.post(`/accounting/payments/${id}/receipt`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export const fetchSummary = async (
  since?: string,
  until?: string
): Promise<AccountingSummary> => {
  const { data } = await axiosInstance.get('/accounting/summary', {
    params: { since, until },
  })
  const parsed = AccountingSummaryResponseSchema.parse(data)
  return parsed.data
}

export const fetchSellableResources = async (
  protocol: string
): Promise<SellableResource[]> => {
  const { data } = await axiosInstance.get('/accounting/sellable-resources', {
    params: { protocol },
  })
  const parsed = SellableResourcesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserProfitability = async (
  since?: string,
  until?: string
): Promise<UserProfitability[]> => {
  const { data } = await axiosInstance.get('/accounting/profitability/users', {
    params: { since, until },
  })
  const parsed = UserProfitabilityResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchLocationProfitability = async (
  since?: string,
  until?: string
): Promise<LocationProfitability[]> => {
  const { data } = await axiosInstance.get('/accounting/profitability/locations', {
    params: { since, until },
  })
  const parsed = LocationProfitabilityResponseSchema.parse(data)
  return parsed.data || []
}
