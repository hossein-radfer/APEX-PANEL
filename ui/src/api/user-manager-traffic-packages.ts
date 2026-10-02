import {
  CreateUserManagerTrafficPackageRequest,
  CreateUserManagerTrafficPackageSchema,
  UserManagerPackagePurchase,
  UserManagerPackagePurchaseResponseSchema,
  UserManagerPackagePurchasesResponseSchema,
  UserManagerTrafficPackage,
  UserManagerTrafficPackageResponseSchema,
  UserManagerTrafficPackagesResponseSchema,
  UpdateUserManagerTrafficPackageRequest,
} from '@/schema/user-manager-traffic-package.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchActiveUserManagerTrafficPackages = async (): Promise<
  UserManagerTrafficPackage[]
> => {
  const { data } = await axiosInstance.get('/user-manager-traffic-package')
  const parsed = UserManagerTrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchAllUserManagerTrafficPackages = async (): Promise<
  UserManagerTrafficPackage[]
> => {
  const { data } = await axiosInstance.get('/user-manager-traffic-package', {
    params: { all: 'true' },
  })
  const parsed = UserManagerTrafficPackagesResponseSchema.parse(data)
  return parsed.data || []
}

export const createUserManagerTrafficPackage = async (
  pkg: CreateUserManagerTrafficPackageRequest
): Promise<UserManagerTrafficPackage> => {
  const validated = CreateUserManagerTrafficPackageSchema.parse(pkg)
  const { data } = await axiosInstance.post(
    '/user-manager-traffic-package',
    validated
  )
  const parsed = UserManagerTrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const updateUserManagerTrafficPackage = async ({
  id,
  ...pkg
}: UpdateUserManagerTrafficPackageRequest & {
  id: number
}): Promise<UserManagerTrafficPackage> => {
  const { data } = await axiosInstance.put(
    `/user-manager-traffic-package/${id}`,
    pkg
  )
  const parsed = UserManagerTrafficPackageResponseSchema.parse(data)
  return parsed.data
}

export const deleteUserManagerTrafficPackage = async (
  id: number
): Promise<void> => {
  await axiosInstance.delete(`/user-manager-traffic-package/${id}`)
}

export const purchaseUserManagerTrafficPackage = async (
  trafficPackageId: number
): Promise<UserManagerPackagePurchase> => {
  const { data } = await axiosInstance.post(
    '/user-manager-traffic-package/purchase',
    { traffic_package_id: trafficPackageId }
  )
  const parsed = UserManagerPackagePurchaseResponseSchema.parse(data)
  return parsed.data
}

export const fetchMyUserManagerPackagePurchases = async (): Promise<
  UserManagerPackagePurchase[]
> => {
  const { data } = await axiosInstance.get(
    '/user-manager-traffic-package/purchases/self'
  )
  const parsed = UserManagerPackagePurchasesResponseSchema.parse(data)
  return parsed.data || []
}
