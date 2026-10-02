import {
  CreateV2RayTrafficPackageRequest,
  CreateV2RayTrafficPackageSchema,
  UpdateV2RayTrafficPackageRequest,
  V2RayPackagePurchase,
  V2RayPackagePurchaseResponseSchema,
  V2RayPackagePurchasesResponseSchema,
  V2RayTrafficPackage,
  V2RayTrafficPackageResponseSchema,
  V2RayTrafficPackagesResponseSchema,
} from '@/schema/v2ray-traffic-package.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchActiveV2RayTrafficPackages = async (): Promise<
  V2RayTrafficPackage[]
> => {
  const { data } = await axiosInstance.get('/v2ray-traffic-package')
  const parsed = V2RayTrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchAllV2RayTrafficPackages = async (): Promise<
  V2RayTrafficPackage[]
> => {
  const { data } = await axiosInstance.get('/v2ray-traffic-package', {
    params: { all: 'true' },
  })
  const parsed = V2RayTrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const createV2RayTrafficPackage = async (
  pkg: CreateV2RayTrafficPackageRequest
): Promise<V2RayTrafficPackage> => {
  const validated = CreateV2RayTrafficPackageSchema.parse(pkg)
  const { data } = await axiosInstance.post(
    '/v2ray-traffic-package',
    validated
  )
  const parsed = V2RayTrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const updateV2RayTrafficPackage = async ({
  id,
  ...pkg
}: UpdateV2RayTrafficPackageRequest & {
  id: number
}): Promise<V2RayTrafficPackage> => {
  const { data } = await axiosInstance.put(
    `/v2ray-traffic-package/${id}`,
    pkg
  )
  const parsed = V2RayTrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const deleteV2RayTrafficPackage = async (
  id: number
): Promise<void> => {
  await axiosInstance.delete(`/v2ray-traffic-package/${id}`)
}

export const purchaseV2RayTrafficPackage = async (
  trafficPackageId: number
): Promise<V2RayPackagePurchase> => {
  const { data } = await axiosInstance.post(
    '/v2ray-traffic-package/purchase',
    { traffic_package_id: trafficPackageId }
  )
  const parsed = V2RayPackagePurchaseResponseSchema.parse(data)
  return parsed.data
}

export const fetchMyV2RayPackagePurchases = async (): Promise<
  V2RayPackagePurchase[]
> => {
  const { data } = await axiosInstance.get(
    '/v2ray-traffic-package/purchases/self'
  )
  const parsed = V2RayPackagePurchasesResponseSchema.parse(data)
  return parsed.data || []
}
