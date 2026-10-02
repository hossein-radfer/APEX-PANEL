import {
  AssignedInterfaceIds,
  AssignedInterfaceIdsResponseSchema,
  AssignedUserManagerGroups,
  AssignedUserManagerGroupsResponseSchema,
  AssignedUserManagerProfiles,
  AssignedUserManagerProfilesResponseSchema,
  CreateResellerRequest,
  CreateResellerSchema,
  Reseller,
  ResellerBillingPrice,
  ResellerBillingPricesResponseSchema,
  ResellerBillingTier,
  ResellerBillingTiersResponseSchema,
  ResellerResponseSchema,
  ResellersResponseSchema,
  UpdateResellerRequest,
} from '@/schema/reseller.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchResellers = async (): Promise<Reseller[]> => {
  const { data } = await axiosInstance.get('/reseller')
  const parsed = ResellersResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchReseller = async (id: number): Promise<Reseller> => {
  const { data } = await axiosInstance.get(`/reseller/${id}`)
  const parsed = ResellerResponseSchema.parse(data)
  return parsed.data
}

export const createReseller = async (
  reseller: CreateResellerRequest
): Promise<Reseller> => {
  const validated = CreateResellerSchema.parse(reseller)
  const { data } = await axiosInstance.post('/reseller', validated)
  const parsed = ResellerResponseSchema.parse(data)
  return parsed.data
}

export const updateReseller = async (
  reseller: UpdateResellerRequest
): Promise<Reseller> => {
  const { data } = await axiosInstance.put(`/reseller/${reseller.id}`, reseller)
  const parsed = ResellerResponseSchema.parse(data)
  return parsed.data
}

export const deleteReseller = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/reseller/${id}`)
}

export const completeResellerOnboarding = async (
  id: number
): Promise<Reseller> => {
  const { data } = await axiosInstance.post(
    `/reseller/${id}/onboarding-complete`
  )
  const parsed = ResellerResponseSchema.parse(data)
  return parsed.data
}

export const fetchAssignedInterfaces = async (
  resellerId: number
): Promise<AssignedInterfaceIds> => {
  const { data } = await axiosInstance.get(`/reseller/${resellerId}/interfaces`)
  const parsed = AssignedInterfaceIdsResponseSchema.parse(data)
  return parsed.data || []
}

export const setAssignedInterfaces = async ({
  resellerId,
  interfaceIds,
}: {
  resellerId: number
  interfaceIds: number[]
}): Promise<void> => {
  await axiosInstance.put(`/reseller/${resellerId}/interfaces`, {
    interfaceIds,
  })
}

export const fetchResellerBillingPrices = async (
  resellerId: number
): Promise<ResellerBillingPrice[]> => {
  const { data } = await axiosInstance.get(
    `/reseller/${resellerId}/billing-prices`
  )
  const parsed = ResellerBillingPricesResponseSchema.parse(data)
  return parsed.data || []
}

export interface ResellerBillingPriceEntry {
  product: string
  locationKey: string
  pricePerGbAmount: number
}

export const setResellerBillingPrices = async ({
  resellerId,
  prices,
}: {
  resellerId: number
  prices: ResellerBillingPriceEntry[]
}): Promise<void> => {
  await axiosInstance.put(`/reseller/${resellerId}/billing-prices`, { prices })
}

export const fetchResellerBillingTiers = async (
  resellerId: number
): Promise<ResellerBillingTier[]> => {
  const { data } = await axiosInstance.get(
    `/reseller/${resellerId}/billing-tiers`
  )
  const parsed = ResellerBillingTiersResponseSchema.parse(data)
  return parsed.data || []
}

export interface ResellerBillingTierEntry {
  minGb: number
  maxGb: number | null
  pricePerGbAmount: number
}

export const setResellerBillingTiers = async ({
  resellerId,
  product,
  tiers,
}: {
  resellerId: number
  product: string
  tiers: ResellerBillingTierEntry[]
}): Promise<void> => {
  await axiosInstance.put(`/reseller/${resellerId}/billing-tiers`, {
    product,
    tiers,
  })
}

export const fetchAssignedUserManagerGroups = async (
  resellerId: number
): Promise<AssignedUserManagerGroups> => {
  const { data } = await axiosInstance.get(
    `/reseller/${resellerId}/user-manager-groups`
  )
  const parsed = AssignedUserManagerGroupsResponseSchema.parse(data)
  return parsed.data || []
}

export const setAssignedUserManagerGroups = async ({
  resellerId,
  groupNames,
}: {
  resellerId: number
  groupNames: string[]
}): Promise<void> => {
  await axiosInstance.put(`/reseller/${resellerId}/user-manager-groups`, {
    groupNames,
  })
}

export const fetchAssignedUserManagerProfiles = async (
  resellerId: number
): Promise<AssignedUserManagerProfiles> => {
  const { data } = await axiosInstance.get(
    `/reseller/${resellerId}/user-manager-profiles`
  )
  const parsed = AssignedUserManagerProfilesResponseSchema.parse(data)
  return parsed.data || []
}

export const setAssignedUserManagerProfiles = async ({
  resellerId,
  profileNames,
}: {
  resellerId: number
  profileNames: string[]
}): Promise<void> => {
  await axiosInstance.put(`/reseller/${resellerId}/user-manager-profiles`, {
    profileNames,
  })
}
