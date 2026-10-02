import {
  CreateTrafficPackageRequest,
  CreateTrafficPackageSchema,
  PackagePurchase,
  PackagePurchaseResponseSchema,
  PackagePurchasesResponseSchema,
  TrafficPackage,
  TrafficPackageResponseSchema,
  TrafficPackagesResponseSchema,
  UpdateTrafficPackageRequest,
} from '@/schema/traffic-package.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchActiveTrafficPackages = async (): Promise<TrafficPackage[]> => {
  const { data } = await axiosInstance.get('/traffic-package')
  const parsed = TrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchAllTrafficPackages = async (): Promise<TrafficPackage[]> => {
  const { data } = await axiosInstance.get('/traffic-package', {
    params: { all: 'true' },
  })
  const parsed = TrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const createTrafficPackage = async (
  pkg: CreateTrafficPackageRequest
): Promise<TrafficPackage> => {
  const validated = CreateTrafficPackageSchema.parse(pkg)
  const { data } = await axiosInstance.post('/traffic-package', validated)
  const parsed = TrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const updateTrafficPackage = async ({
  id,
  ...pkg
}: UpdateTrafficPackageRequest & { id: number }): Promise<TrafficPackage> => {
  const { data } = await axiosInstance.put(`/traffic-package/${id}`, pkg)
  const parsed = TrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const deleteTrafficPackage = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/traffic-package/${id}`)
}

export const purchaseTrafficPackage = async (
  trafficPackageId: number
): Promise<PackagePurchase> => {
  const { data } = await axiosInstance.post('/traffic-package/purchase', {
    traffic_package_id: trafficPackageId,
  })
  const parsed = PackagePurchaseResponseSchema.parse(data)
  return parsed.data
}

export const fetchMyPackagePurchases = async (): Promise<PackagePurchase[]> => {
  const { data } = await axiosInstance.get('/traffic-package/purchases/self')
  const parsed = PackagePurchasesResponseSchema.parse(data)
  return parsed.data || []
}
