import {
  CreateDNSPlanRequest,
  CreateDNSPlanSchema,
  DNSPlan,
  DNSPlanResponseSchema,
  DNSPlansResponseSchema,
  UpdateDNSPlanRequest,
} from '@/schema/dns-plan.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchDNSPlans = async (): Promise<DNSPlan[]> => {
  const { data } = await axiosInstance.get('/dns-plan')
  const parsed = DNSPlansResponseSchema.parse(data)
  return parsed.data || []
}

export const createDNSPlan = async (plan: CreateDNSPlanRequest): Promise<DNSPlan> => {
  const validated = CreateDNSPlanSchema.parse(plan)
  const { data } = await axiosInstance.post('/dns-plan', validated)
  const parsed = DNSPlanResponseSchema.parse(data)
  return parsed.data
}

export const updateDNSPlan = async ({
  id,
  ...plan
}: UpdateDNSPlanRequest & { id: number }): Promise<DNSPlan> => {
  const { data } = await axiosInstance.put(`/dns-plan/${id}`, plan)
  const parsed = DNSPlanResponseSchema.parse(data)
  return parsed.data
}

export const deleteDNSPlan = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/dns-plan/${id}`)
}
