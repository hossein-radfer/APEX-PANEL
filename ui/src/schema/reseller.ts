import { z } from 'zod'
import { createApiResponseSchema } from '@/schema/api-response.ts'

export const BYTES_PER_GB = 1024 * 1024 * 1024

const optionalEmail = z.preprocess(
  (value) => {
    if (value === '' || value === null || value === undefined) {
      return undefined
    }

    return value
  },
  z.string().email('Email must be valid').optional()
)

const optionalNonNegativeNumber = z.preprocess(
  (value) => {
    if (value === '' || value === null || value === undefined) {
      return undefined
    }

    if (typeof value === 'number') {
      return value
    }

    return Number(value)
  },
  z.number().nonnegative().optional()
)

const optionalPositiveInt = z.preprocess(
  (value) => {
    if (value === '' || value === null || value === undefined) {
      return undefined
    }

    if (typeof value === 'number') {
      return value
    }

    return Number(value)
  },
  z.number().int().positive().optional()
)

const optionalTelegramChatId = z.preprocess((value) => {
  if (value === '' || value === null || value === undefined) {
    return undefined
  }
  return value
}, z.string().optional())

export const ResellerSchema = z.object({
  id: z.number(),
  name: z.string(),
  username: z.string(),
  email: z.string().nullable().optional(),
  isActive: z.boolean(),
  // A confirmed, reported bug: the backend's QuotaBytes is a *int64 with
  // no `omitempty` tag, so an unlimited-quota reseller serializes this as
  // literal JSON `null`, not an omitted key -- missing .nullable() here
  // meant EVERY reseller create/update/list response for any reseller
  // with no quota set failed Zod's .parse() outright, which is what
  // actually produced the "Failed to save reseller" toast (the reseller
  // was genuinely created server-side; the response just couldn't be
  // parsed back) and the resellers list silently showing empty/"No
  // results" despite resellers existing in the database. Every sibling
  // field on this same schema (maxPeers, userManagerQuotaBytes, etc.)
  // already correctly has .nullable() -- this one was simply missed.
  quotaBytes: z.number().nullable().optional(),
  usedBytes: z.number(),
  maxPeers: z.number().nullable().optional(),
  peerCount: z.number().optional(),
  telegramChatId: z.string().nullable().optional(),
  otpEnabled: z.boolean().optional(),

  canCreateUserManagerAccounts: z.boolean().optional(),
  userManagerQuotaBytes: z.number().nullable().optional(),
  userManagerUsedBytes: z.number().optional(),
  userManagerMaxAccounts: z.number().nullable().optional(),
  userManagerAccountCount: z.number().optional(),

  canResellV2Ray: z.boolean().optional(),
  v2rayQuotaBytes: z.number().nullable().optional(),
  v2rayUsedBytes: z.number().optional(),
  v2rayMaxPackages: z.number().nullable().optional(),
  v2rayPackageCount: z.number().optional(),

  canResellDns: z.boolean().optional(),
  dnsQuotaBytes: z.number().nullable().optional(),
  dnsUsedBytes: z.number().optional(),
  dnsMaxAccounts: z.number().nullable().optional(),
  dnsAccountCount: z.number().optional(),

  canCreateApplications: z.boolean().optional(),
  applicationQuotaBytes: z.number().nullable().optional(),
  applicationUsedBytes: z.number().optional(),
  applicationMaxCount: z.number().nullable().optional(),
  applicationCount: z.number().optional(),

  billingMode: z.enum(['VOLUME', 'PAYMENT']).optional(),
  paymentSubMode: z.enum(['PREPAID', 'POSTPAID']).optional(),
  debtLimitAmount: z.number().nullable().optional(),
  billingSuspended: z.boolean().optional(),

  hasCompletedOnboarding: z.boolean().optional(),
})

export const ResellersSchema = z.array(ResellerSchema).nullable()

export const CreateResellerSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  username: z.string().min(3, 'Username must be at least 3 characters'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  email: optionalEmail,
  quotaBytes: optionalNonNegativeNumber,
  maxPeers: optionalPositiveInt,
  telegramChatId: optionalTelegramChatId,
  otpEnabled: z.boolean().optional(),

  canCreateUserManagerAccounts: z.boolean().optional(),
  userManagerQuotaBytes: optionalNonNegativeNumber,
  userManagerMaxAccounts: optionalPositiveInt,

  canResellV2Ray: z.boolean().optional(),
  v2rayQuotaBytes: optionalNonNegativeNumber,
  v2rayMaxPackages: optionalPositiveInt,

  canResellDns: z.boolean().optional(),
  dnsQuotaBytes: optionalNonNegativeNumber,
  dnsMaxAccounts: optionalPositiveInt,

  canCreateApplications: z.boolean().optional(),
  applicationQuotaBytes: optionalNonNegativeNumber,
  applicationMaxCount: optionalPositiveInt,

  billingMode: z.enum(['VOLUME', 'PAYMENT']).optional(),
  paymentSubMode: z.enum(['PREPAID', 'POSTPAID']).optional(),
  debtLimitAmount: optionalNonNegativeNumber,
})

export const UpdateResellerSchema = z.object({
  id: z.number().int().positive(),
  name: z.string().min(1, 'Name is required'),
  username: z
    .string()
    .min(3, 'Username must be at least 3 characters')
    .optional(),
  password: z
    .string()
    .min(8, 'Password must be at least 8 characters')
    .optional()
    .or(z.literal('')),
  email: optionalEmail,
  isActive: z.boolean().optional(),
  quotaBytes: optionalNonNegativeNumber,
  maxPeers: optionalPositiveInt,
  clearMaxPeers: z.boolean().optional(),
  telegramChatId: optionalTelegramChatId,
  otpEnabled: z.boolean().optional(),

  canCreateUserManagerAccounts: z.boolean().optional(),
  userManagerQuotaBytes: optionalNonNegativeNumber,
  userManagerMaxAccounts: optionalPositiveInt,
  clearUserManagerMaxAccounts: z.boolean().optional(),

  canResellV2Ray: z.boolean().optional(),
  v2rayQuotaBytes: optionalNonNegativeNumber,
  v2rayMaxPackages: optionalPositiveInt,
  clearV2RayMaxPackages: z.boolean().optional(),

  canResellDns: z.boolean().optional(),
  dnsQuotaBytes: optionalNonNegativeNumber,
  dnsMaxAccounts: optionalPositiveInt,
  clearDnsMaxAccounts: z.boolean().optional(),

  canCreateApplications: z.boolean().optional(),
  applicationQuotaBytes: optionalNonNegativeNumber,
  applicationMaxCount: optionalPositiveInt,
  clearApplicationMaxCount: z.boolean().optional(),

  billingMode: z.enum(['VOLUME', 'PAYMENT']).optional(),
  paymentSubMode: z.enum(['PREPAID', 'POSTPAID']).optional(),
  debtLimitAmount: optionalNonNegativeNumber,
  clearDebtLimitAmount: z.boolean().optional(),
})

export const ResellerResponseSchema = createApiResponseSchema(ResellerSchema)
export const ResellersResponseSchema = createApiResponseSchema(ResellersSchema)

export const ResellerBillingPriceSchema = z.object({
  product: z.enum(['WIREGUARD', 'USER_MANAGER', 'V2RAY', 'APPLICATION']),
  locationKey: z.string(),
  pricePerGbAmount: z.number(),
})
export const ResellerBillingPricesSchema = z.array(ResellerBillingPriceSchema)
export const ResellerBillingPricesResponseSchema = createApiResponseSchema(
  ResellerBillingPricesSchema
)
export type ResellerBillingPrice = z.infer<typeof ResellerBillingPriceSchema>
export type ResellerBillingProduct = ResellerBillingPrice['product']

export const ResellerBillingTierSchema = z.object({
  product: z.enum(['WIREGUARD', 'USER_MANAGER', 'V2RAY', 'APPLICATION']),
  minGb: z.number(),
  maxGb: z.number().nullable(),
  pricePerGbAmount: z.number(),
})
export const ResellerBillingTiersSchema = z.array(ResellerBillingTierSchema)
export const ResellerBillingTiersResponseSchema = createApiResponseSchema(
  ResellerBillingTiersSchema
)
export type ResellerBillingTier = z.infer<typeof ResellerBillingTierSchema>

export const AssignedInterfaceIdsSchema = z.array(z.number()).nullable()
export const AssignedInterfaceIdsResponseSchema = createApiResponseSchema(
  AssignedInterfaceIdsSchema
)

export const AssignedUserManagerGroupsSchema = z.array(z.string()).nullable()
export const AssignedUserManagerGroupsResponseSchema = createApiResponseSchema(
  AssignedUserManagerGroupsSchema
)
export const AssignedUserManagerProfilesSchema = z.array(z.string()).nullable()
export const AssignedUserManagerProfilesResponseSchema =
  createApiResponseSchema(AssignedUserManagerProfilesSchema)

export type ResellerResponse = z.infer<typeof ResellerSchema>
export type Reseller = z.infer<typeof ResellerSchema>
export type CreateResellerRequest = z.infer<typeof CreateResellerSchema>
export type UpdateResellerRequest = z.infer<typeof UpdateResellerSchema>
export type AssignedInterfaceIds = z.infer<typeof AssignedInterfaceIdsSchema>
export type AssignedUserManagerGroups = z.infer<
  typeof AssignedUserManagerGroupsSchema
>
export type AssignedUserManagerProfiles = z.infer<
  typeof AssignedUserManagerProfilesSchema
>
