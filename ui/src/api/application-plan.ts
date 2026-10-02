import {
  ApplicationPlan,
  ApplicationPlanResponseSchema,
  ApplicationPlansResponseSchema,
  CreateApplicationPlanRequest,
  CreateApplicationPlanSchema,
  UpdateApplicationPlanRequest,
} from '@/schema/application-plan.ts'
import axiosInstance from '@/api/axios-instance.ts'

// GET /application-plan is reachable by both admin and reseller sessions
// and returns only active plans by default -- unlike fetchDNSPlans (admin
// -only), a reseller needs this to populate their own Application form's
// plan picker. fetchAllApplicationPlans (?all=true) is admin-only and also
// surfaces inactive plans, used by the plan management page.
export const fetchActiveApplicationPlans = async (): Promise<ApplicationPlan[]> => {
  const { data } = await axiosInstance.get('/application-plan')
  const parsed = ApplicationPlansResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchAllApplicationPlans = async (): Promise<ApplicationPlan[]> => {
  const { data } = await axiosInstance.get('/application-plan', { params: { all: true } })
  const parsed = ApplicationPlansResponseSchema.parse(data)
  return parsed.data || []
}

export const createApplicationPlan = async (
  plan: CreateApplicationPlanRequest
): Promise<ApplicationPlan> => {
  const validated = CreateApplicationPlanSchema.parse(plan)
  const { data } = await axiosInstance.post('/application-plan', validated)
  const parsed = ApplicationPlanResponseSchema.parse(data)
  return parsed.data
}

export const updateApplicationPlan = async ({
  id,
  ...plan
}: UpdateApplicationPlanRequest & { id: number }): Promise<ApplicationPlan> => {
  const { data } = await axiosInstance.put(`/application-plan/${id}`, plan)
  const parsed = ApplicationPlanResponseSchema.parse(data)
  return parsed.data
}

export const deleteApplicationPlan = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/application-plan/${id}`)
}
