import {
  BulkCreatedUserManagerAccount,
  BulkCreateUserManagerAccountRequest,
  BulkCreateUserManagerAccountResponseSchema,
  BulkCreateUserManagerAccountSchema,
  BulkImportAccountsResponseSchema,
  BulkImportAccountsResult,
  CreateUserManagerAccountRequest,
  CreateUserManagerAccountSchema,
  UpdateUserManagerAccountRequest,
  UpdateUserManagerAccountShareExpireRequest,
  UpsertUserManagerProtocolConfigRequest,
  UpsertUserManagerProtocolConfigSchema,
  UserManagerAccount,
  UserManagerAccountResponseSchema,
  UserManagerAccountShare,
  UserManagerAccountShareDetails,
  UserManagerAccountShareDetailsResponseSchema,
  UserManagerAccountShareResponseSchema,
  UserManagerAccountsResponseSchema,
  UserManagerGroup,
  UserManagerGroupsResponseSchema,
  UserManagerProfile,
  UserManagerProfilesResponseSchema,
  ProtocolConfigProtocol,
  UserManagerProtocolConfig,
  UserManagerProtocolConfigResponseSchema,
  UserManagerProtocolConfigsResponseSchema,
  UserManagerSelfSummary,
  UserManagerSelfSummaryResponseSchema,
} from '@/schema/user-manager.ts'
import axiosInstance from '@/api/axios-instance.ts'
import { parseFilenameFromContentDisposition } from '@/lib/download.ts'

export interface DownloadedFile {
  blob: Blob
  filename: string | null
}

export const fetchUserManagerAccountsList = async (): Promise<
  UserManagerAccount[]
> => {
  const { data } = await axiosInstance.get('/user-manager/account')
  const parsed = UserManagerAccountsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserManagerSelfSummary =
  async (): Promise<UserManagerSelfSummary> => {
    const { data } = await axiosInstance.get('/user-manager/account/summary/self')
    const parsed = UserManagerSelfSummaryResponseSchema.parse(data)
    return parsed.data
  }

export const fetchUserManagerAccountsByReseller = async (
  resellerId: number
): Promise<UserManagerAccount[]> => {
  const { data } = await axiosInstance.get(
    `/user-manager/account/reseller/${resellerId}`
  )
  const parsed = UserManagerAccountsResponseSchema.parse(data)
  return parsed.data || []
}

export const createUserManagerAccount = async (
  account: CreateUserManagerAccountRequest
): Promise<UserManagerAccount> => {
  const validated = CreateUserManagerAccountSchema.parse(account)
  const { data } = await axiosInstance.post('/user-manager/account', validated)
  const parsed = UserManagerAccountResponseSchema.parse(data)
  return parsed.data
}

export const bulkCreateUserManagerAccounts = async (
  req: BulkCreateUserManagerAccountRequest
): Promise<BulkCreatedUserManagerAccount[]> => {
  const validated = BulkCreateUserManagerAccountSchema.parse(req)
  const { data } = await axiosInstance.post('/user-manager/account/bulk', validated)
  const parsed = BulkCreateUserManagerAccountResponseSchema.parse(data)
  return parsed.data.accounts || []
}

export const createUserManagerAccountForReseller = async ({
  resellerId,
  account,
}: {
  resellerId: number
  account: CreateUserManagerAccountRequest
}): Promise<UserManagerAccount> => {
  const validated = CreateUserManagerAccountSchema.parse(account)
  const { data } = await axiosInstance.post(
    `/user-manager/account/reseller/${resellerId}`,
    validated
  )
  const parsed = UserManagerAccountResponseSchema.parse(data)
  return parsed.data
}

export const bulkImportUserManagerAccounts = async (
  file: File
): Promise<BulkImportAccountsResult> => {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await axiosInstance.post(
    '/user-manager/account/bulk-import',
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
  const parsed = BulkImportAccountsResponseSchema.parse(data)
  return parsed.data
}

export const bulkImportUserManagerAccountsForReseller = async ({
  resellerId,
  file,
}: {
  resellerId: number
  file: File
}): Promise<BulkImportAccountsResult> => {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await axiosInstance.post(
    `/user-manager/account/reseller/${resellerId}/bulk-import`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
  const parsed = BulkImportAccountsResponseSchema.parse(data)
  return parsed.data
}

export const updateUserManagerAccount = async (
  account: UpdateUserManagerAccountRequest
): Promise<UserManagerAccount> => {
  const { data } = await axiosInstance.put(
    `/user-manager/account/${account.id}`,
    account
  )
  const parsed = UserManagerAccountResponseSchema.parse(data)
  return parsed.data
}

export const updateUserManagerAccountForReseller = async ({
  resellerId,
  account,
}: {
  resellerId: number
  account: UpdateUserManagerAccountRequest
}): Promise<UserManagerAccount> => {
  const { data } = await axiosInstance.put(
    `/user-manager/account/reseller/${resellerId}/${account.id}`,
    account
  )
  const parsed = UserManagerAccountResponseSchema.parse(data)
  return parsed.data
}

export const deleteUserManagerAccount = async (id: number): Promise<void> => {
  await axiosInstance.delete(`/user-manager/account/${id}`)
}

export const resetUserManagerAccountUsage = async (id: number): Promise<void> => {
  await axiosInstance.patch(`/user-manager/account/${id}/reset-usage`)
}

export const bulkDeleteUserManagerAccounts = async (
  ids: number[]
): Promise<{ deleted: number[]; failedCount: number }> => {
  const { data } = await axiosInstance.post('/user-manager/account/bulk-delete', { ids })
  return {
    deleted: data?.data?.deleted ?? [],
    failedCount: data?.data?.failed?.length ?? 0,
  }
}

export const deleteUserManagerAccountForReseller = async ({
  resellerId,
  accountId,
}: {
  resellerId: number
  accountId: number
}): Promise<void> => {
  await axiosInstance.delete(
    `/user-manager/account/reseller/${resellerId}/${accountId}`
  )
}

export const updateUserManagerAccountStatus = async (
  id: number
): Promise<void> => {
  await axiosInstance.patch(`/user-manager/account/${id}/status`)
}

export const updateUserManagerAccountStatusForReseller = async ({
  resellerId,
  accountId,
}: {
  resellerId: number
  accountId: number
}): Promise<void> => {
  await axiosInstance.patch(
    `/user-manager/account/reseller/${resellerId}/${accountId}/status`
  )
}

export const changeUserManagerAccountPassword = async ({
  id,
  password,
}: {
  id: number
  password: string
}): Promise<void> => {
  await axiosInstance.patch(`/user-manager/account/${id}/password`, { password })
}

export const changeUserManagerAccountPasswordForReseller = async ({
  resellerId,
  accountId,
  password,
}: {
  resellerId: number
  accountId: number
  password: string
}): Promise<void> => {
  await axiosInstance.patch(
    `/user-manager/account/reseller/${resellerId}/${accountId}/password`,
    { password }
  )
}

export const fetchUserManagerAccountShareStatus = async (
  id: number
): Promise<UserManagerAccountShare> => {
  const { data } = await axiosInstance.get(`/user-manager/account/${id}/share`)
  const parsed = UserManagerAccountShareResponseSchema.parse(data)
  return parsed.data
}

export const updateUserManagerAccountShareStatus = async (
  id: number
): Promise<void> => {
  await axiosInstance.patch(`/user-manager/account/${id}/share/status`)
}

export const updateUserManagerAccountShareExpire = async (
  account: UpdateUserManagerAccountShareExpireRequest
): Promise<void> => {
  await axiosInstance.patch(
    `/user-manager/account/${account.id}/share/expire`,
    account
  )
}

export const fetchUserManagerGroups = async (): Promise<
  UserManagerGroup[]
> => {
  const { data } = await axiosInstance.get('/user-manager/group')
  const parsed = UserManagerGroupsResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserManagerProfiles = async (): Promise<
  UserManagerProfile[]
> => {
  const { data } = await axiosInstance.get('/user-manager/profile')
  const parsed = UserManagerProfilesResponseSchema.parse(data)
  return parsed.data || []
}

export const fetchUserManagerProtocolConfigs = async (): Promise<
  UserManagerProtocolConfig[]
> => {
  const { data } = await axiosInstance.get('/user-manager/protocol-config')
  const parsed = UserManagerProtocolConfigsResponseSchema.parse(data)
  return parsed.data || []
}

export const upsertUserManagerProtocolConfig = async (
  config: UpsertUserManagerProtocolConfigRequest
): Promise<UserManagerProtocolConfig> => {
  const validated = UpsertUserManagerProtocolConfigSchema.parse(config)
  const { data } = await axiosInstance.put(
    '/user-manager/protocol-config',
    validated
  )
  const parsed = UserManagerProtocolConfigResponseSchema.parse(data)
  return parsed.data
}

export const fetchUserManagerAccountShareDetails = async (
  uuid: string
): Promise<UserManagerAccountShareDetails> => {
  const { data } = await axiosInstance.get(
    `/user/${uuid}/user-manager-account`
  )
  const parsed = UserManagerAccountShareDetailsResponseSchema.parse(data)
  return parsed.data
}

export const uploadUserManagerAccountConfig = async ({
  accountId,
  file,
}: {
  accountId: number
  file: File
}): Promise<void> => {
  const formData = new FormData()
  formData.append('config', file)
  await axiosInstance.post(`/user-manager/account/${accountId}/config`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export const uploadUserManagerAccountConfigForReseller = async ({
  resellerId,
  accountId,
  file,
}: {
  resellerId: number
  accountId: number
  file: File
}): Promise<void> => {
  const formData = new FormData()
  formData.append('config', file)
  await axiosInstance.post(
    `/user-manager/account/reseller/${resellerId}/${accountId}/config`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
}

export const fetchUserManagerAccountConfig = async (
  accountId: number
): Promise<Blob> => {
  const response = await axiosInstance.get(
    `/user-manager/account/${accountId}/config`,
    { responseType: 'blob' }
  )
  return response.data
}

export const fetchUserManagerAccountConfigPublic = async (
  uuid: string
): Promise<Blob> => {
  const response = await axiosInstance.get(
    `/user/${uuid}/user-manager-account/config`,
    { responseType: 'blob' }
  )
  return response.data
}

export const uploadUserManagerProtocolCertificateFile = async ({
  protocol,
  file,
}: {
  protocol: ProtocolConfigProtocol
  file: File
}): Promise<void> => {
  const formData = new FormData()
  formData.append('file', file)
  await axiosInstance.post(
    `/user-manager/protocol-config/${protocol}/certificate`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
}

export const uploadUserManagerProtocolClientAppFile = async ({
  protocol,
  file,
}: {
  protocol: ProtocolConfigProtocol
  file: File
}): Promise<void> => {
  const formData = new FormData()
  formData.append('file', file)
  await axiosInstance.post(
    `/user-manager/protocol-config/${protocol}/client-app`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
}

export const deleteUserManagerProtocolCertificateFile = async (
  protocol: ProtocolConfigProtocol
): Promise<void> => {
  await axiosInstance.delete(
    `/user-manager/protocol-config/${protocol}/certificate`
  )
}

export const deleteUserManagerProtocolClientAppFile = async (
  protocol: ProtocolConfigProtocol
): Promise<void> => {
  await axiosInstance.delete(
    `/user-manager/protocol-config/${protocol}/client-app`
  )
}

export const fetchUserManagerProtocolCertificateFile = async (
  protocol: ProtocolConfigProtocol
): Promise<DownloadedFile> => {
  const response = await axiosInstance.get(
    `/user-manager/protocol-config/${protocol}/certificate`,
    { responseType: 'blob' }
  )
  return {
    blob: response.data,
    filename: parseFilenameFromContentDisposition(
      response.headers['content-disposition']
    ),
  }
}

export const fetchUserManagerProtocolClientAppFile = async (
  protocol: ProtocolConfigProtocol
): Promise<DownloadedFile> => {
  const response = await axiosInstance.get(
    `/user-manager/protocol-config/${protocol}/client-app`,
    { responseType: 'blob' }
  )
  return {
    blob: response.data,
    filename: parseFilenameFromContentDisposition(
      response.headers['content-disposition']
    ),
  }
}

export const fetchUserManagerProtocolCertificateFilePublic = async (
  uuid: string,
  protocol: string
): Promise<DownloadedFile> => {
  const response = await axiosInstance.get(
    `/user/${uuid}/user-manager-account/protocol-certificate`,
    { responseType: 'blob', params: { protocol } }
  )
  return {
    blob: response.data,
    filename: parseFilenameFromContentDisposition(
      response.headers['content-disposition']
    ),
  }
}

export const fetchUserManagerProtocolClientAppFilePublic = async (
  uuid: string,
  protocol: string
): Promise<DownloadedFile> => {
  const response = await axiosInstance.get(
    `/user/${uuid}/user-manager-account/protocol-client-app`,
    { responseType: 'blob', params: { protocol } }
  )
  return {
    blob: response.data,
    filename: parseFilenameFromContentDisposition(
      response.headers['content-disposition']
    ),
  }
}
