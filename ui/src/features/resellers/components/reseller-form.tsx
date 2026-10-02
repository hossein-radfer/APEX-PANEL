'use client'

import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  BYTES_PER_GB,
  CreateResellerRequest,
  CreateResellerSchema,
  Reseller,
  UpdateResellerRequest,
  UpdateResellerSchema,
} from '@/schema/reseller.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useCreateResellerMutation } from '@/hooks/resellers/useCreateResellerMutation.ts'
import { useUpdateResellerMutation } from '@/hooks/resellers/useUpdateResellerMutation.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'
import { useAssignedInterfacesQuery } from '@/hooks/resellers/useAssignedInterfacesQuery.ts'
import { useSetAssignedInterfacesMutation } from '@/hooks/resellers/useSetAssignedInterfacesMutation.ts'
import { useAssignedUserManagerGroupsQuery } from '@/hooks/resellers/useAssignedUserManagerGroupsQuery.ts'
import { useSetAssignedUserManagerGroupsMutation } from '@/hooks/resellers/useSetAssignedUserManagerGroupsMutation.ts'
import { useAssignedUserManagerProfilesQuery } from '@/hooks/resellers/useAssignedUserManagerProfilesQuery.ts'
import { useSetAssignedUserManagerProfilesMutation } from '@/hooks/resellers/useSetAssignedUserManagerProfilesMutation.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { useAssignedXuiPanelsQuery } from '@/hooks/v2ray/useAssignedXuiPanelsQuery.ts'
import { useSetAssignedXuiPanelsMutation } from '@/hooks/v2ray/useSetAssignedXuiPanelsMutation.ts'
import { useDNSPanelsListQuery } from '@/hooks/dns-panel/useDNSPanelsListQuery.ts'
import { useAssignedDNSPanelsQuery } from '@/hooks/dns-account/useAssignedDNSPanelsQuery.ts'
import { useSetAssignedDNSPanelsMutation } from '@/hooks/dns-account/useSetAssignedDNSPanelsMutation.ts'
import { useQuery, useMutation } from '@tanstack/react-query'
import {
  fetchResellerBillingPrices,
  setResellerBillingPrices,
} from '@/api/resellers.ts'
import { creditWallet } from '@/api/wallet.ts'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'
import { Button } from '@/components/ui/button.tsx'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import { PasswordInput } from '@/components/password-input.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'

interface Props {
  currentRow?: Partial<Reseller>
  onClose: () => void
  setPending: (pending: boolean) => void
}

export function ResellerForm({ currentRow, onClose, setPending }: Props) {
  const isEdit = Boolean(currentRow?.id)
  const { mutateAsync: createReseller, isPending: isCreatePending } =
    useCreateResellerMutation()
  const { mutateAsync: updateReseller, isPending: isUpdatePending } =
    useUpdateResellerMutation()
  const { mutateAsync: setAssignedInterfaces, isPending: isAssignPending } =
    useSetAssignedInterfacesMutation()
  const {
    mutateAsync: setAssignedUserManagerGroups,
    isPending: isAssignGroupsPending,
  } = useSetAssignedUserManagerGroupsMutation()
  const {
    mutateAsync: setAssignedUserManagerProfiles,
    isPending: isAssignProfilesPending,
  } = useSetAssignedUserManagerProfilesMutation()
  const { mutateAsync: setAssignedXuiPanels, isPending: isAssignPanelsPending } =
    useSetAssignedXuiPanelsMutation()
  const {
    mutateAsync: setAssignedDNSPanels,
    isPending: isAssignDNSPanelsPending,
  } = useSetAssignedDNSPanelsMutation()

  const { data: interfacesList = [] } = useInterfacesListQuery()
  const { data: assignedInterfaceIds } = useAssignedInterfacesQuery(
    currentRow?.id
  )
  const [selectedInterfaceIds, setSelectedInterfaceIds] = useState<number[]>(
    []
  )

  const { data: umGroupsList = [] } = useUserManagerGroupsQuery()
  const { data: umProfilesList = [] } = useUserManagerProfilesQuery()
  const { data: assignedUmGroups } = useAssignedUserManagerGroupsQuery(
    currentRow?.id
  )
  const { data: assignedUmProfiles } = useAssignedUserManagerProfilesQuery(
    currentRow?.id
  )
  const [selectedUmGroups, setSelectedUmGroups] = useState<string[]>([])
  const [selectedUmProfiles, setSelectedUmProfiles] = useState<string[]>([])

  const { data: xuiPanelsList = [] } = useXuiPanelsListQuery()
  const { data: assignedPanelIds } = useAssignedXuiPanelsQuery(currentRow?.id)
  const [selectedPanelIds, setSelectedPanelIds] = useState<number[]>([])

  const { data: dnsPanelsList = [] } = useDNSPanelsListQuery()
  const { data: assignedDNSPanelIds } = useAssignedDNSPanelsQuery(
    currentRow?.id
  )
  const [selectedDNSPanelIds, setSelectedDNSPanelIds] = useState<number[]>([])

  // Payment-based per-GB prices -- previously only reachable via a
  // separate "Billing Prices" action after the reseller already existed,
  // which meant switching a NEW reseller to Payment-based left it
  // uncharged (price rows default to 0) until a second trip to that
  // action. Surfacing the same fields inline here removes that trip
  // entirely: create-mode submits them right after the reseller row is
  // created, edit-mode loads and re-saves them like any other field.
  const { data: existingBillingPrices } = useQuery({
    queryKey: ['reseller_billing_prices', currentRow?.id],
    queryFn: () => fetchResellerBillingPrices(currentRow!.id as number),
    enabled: Boolean(currentRow?.id),
  })
  const [billingPriceAmounts, setBillingPriceAmounts] = useState<
    Record<string, string>
  >({})
  useEffect(() => {
    if (!existingBillingPrices) return
    const next: Record<string, string> = {}
    for (const price of existingBillingPrices) {
      // This inline section only edits the product-wide fallback price
      // (locationKey === '') -- per-location overrides and volume tiers
      // are configured on the dedicated Billing Prices page instead (see
      // the link below), since a location/tier editor needs the
      // reseller's own assigned interfaces/groups/panels loaded, which
      // doesn't belong cluttering this create/edit dialog.
      if (price.locationKey === '') {
        next[price.product] = String(price.pricePerGbAmount)
      }
    }
    setBillingPriceAmounts(next)
  }, [existingBillingPrices])

  // Wallet top-up -- lets the admin credit a reseller's wallet right from
  // the same edit dialog when the reseller has already paid, instead of
  // navigating away to a separate wallet screen. Edit mode only: a wallet
  // is created alongside the reseller row itself, so there is nothing to
  // credit yet during Add New Reseller.
  const [walletTopUpAmount, setWalletTopUpAmount] = useState('')
  const { mutateAsync: creditWalletMutate, isPending: isWalletCreditPending } =
    useMutation({
      mutationFn: (amount: number) =>
        creditWallet(
          currentRow!.id as number,
          amount,
          'Manual top-up from reseller edit form'
        ),
      onSuccess: () => {
        toast.success('کیف پول با موفقیت شارژ شد.')
        setWalletTopUpAmount('')
      },
      onError: (error) =>
        toast.error(getApiErrorMessage(error, 'شارژ کیف پول ناموفق بود.')),
    })

  // Only sync from the server once we actually have an assignment to load
  // (edit mode). On create there is nothing to fetch — assignedInterfaceIds
  // stays undefined forever since the query is disabled, so this must not
  // run on every render or it will stomp the user's in-progress selection.
  useEffect(() => {
    if (assignedInterfaceIds != null) {
      setSelectedInterfaceIds(assignedInterfaceIds)
    }
  }, [assignedInterfaceIds])

  useEffect(() => {
    if (assignedUmGroups != null) {
      setSelectedUmGroups(assignedUmGroups)
    }
  }, [assignedUmGroups])

  useEffect(() => {
    if (assignedUmProfiles != null) {
      setSelectedUmProfiles(assignedUmProfiles)
    }
  }, [assignedUmProfiles])

  useEffect(() => {
    if (assignedPanelIds != null) {
      setSelectedPanelIds(assignedPanelIds)
    }
  }, [assignedPanelIds])

  useEffect(() => {
    if (assignedDNSPanelIds != null) {
      setSelectedDNSPanelIds(assignedDNSPanelIds)
    }
  }, [assignedDNSPanelIds])

  const toggleInterface = (id: number, checked: boolean) => {
    setSelectedInterfaceIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }

  const toggleUmGroup = (name: string, checked: boolean) => {
    setSelectedUmGroups((prev) =>
      checked ? [...prev, name] : prev.filter((existing) => existing !== name)
    )
  }

  const toggleUmProfile = (name: string, checked: boolean) => {
    setSelectedUmProfiles((prev) =>
      checked ? [...prev, name] : prev.filter((existing) => existing !== name)
    )
  }

  const toggleXuiPanel = (id: number, checked: boolean) => {
    setSelectedPanelIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }

  const toggleDNSPanel = (id: number, checked: boolean) => {
    setSelectedDNSPanelIds((prev) =>
      checked ? [...prev, id] : prev.filter((existingId) => existingId !== id)
    )
  }

  const form = useForm<any>({
    resolver: zodResolver(
      isEdit ? (UpdateResellerSchema as any) : (CreateResellerSchema as any)
    ),
    defaultValues: currentRow ?? {
      name: '',
      username: '',
      password: '',
      email: '',
      quotaBytes: undefined,
      maxPeers: undefined,
      isActive: true,
      telegramChatId: '',
      otpEnabled: false,
      canCreateUserManagerAccounts: false,
      userManagerQuotaBytes: undefined,
      userManagerMaxAccounts: undefined,
      canResellV2Ray: false,
      v2rayQuotaBytes: undefined,
      v2rayMaxPackages: undefined,
      canResellDns: false,
      dnsQuotaBytes: undefined,
      dnsMaxAccounts: undefined,
      canCreateApplications: false,
      applicationQuotaBytes: undefined,
      applicationMaxCount: undefined,
      billingMode: 'VOLUME',
      paymentSubMode: 'PREPAID',
      debtLimitAmount: undefined,
    },
  })

  const canCreateUserManagerAccounts = form.watch(
    'canCreateUserManagerAccounts'
  )
  const canResellV2Ray = form.watch('canResellV2Ray')
  const canResellDns = form.watch('canResellDns')
  const canCreateApplications = form.watch('canCreateApplications')
  const billingMode = form.watch('billingMode')
  const paymentSubMode = form.watch('paymentSubMode')

  useEffect(() => {
    if (currentRow) {
      form.reset({
        id: currentRow.id,
        name: currentRow.name,
        username: currentRow.username,
        password: '',
        email: currentRow.email ?? '',
        quotaBytes:
          currentRow.quotaBytes !== null && currentRow.quotaBytes !== undefined
            ? Number((currentRow.quotaBytes / BYTES_PER_GB).toFixed(2))
            : undefined,
        maxPeers: currentRow.maxPeers ?? undefined,
        isActive: currentRow.isActive,
        telegramChatId: currentRow.telegramChatId ?? '',
        otpEnabled: currentRow.otpEnabled ?? false,
        canCreateUserManagerAccounts:
          currentRow.canCreateUserManagerAccounts ?? false,
        userManagerQuotaBytes:
          currentRow.userManagerQuotaBytes !== null &&
          currentRow.userManagerQuotaBytes !== undefined
            ? Number(
                (currentRow.userManagerQuotaBytes / BYTES_PER_GB).toFixed(2)
              )
            : undefined,
        userManagerMaxAccounts: currentRow.userManagerMaxAccounts ?? undefined,
        canResellV2Ray: currentRow.canResellV2Ray ?? false,
        v2rayQuotaBytes:
          currentRow.v2rayQuotaBytes !== null &&
          currentRow.v2rayQuotaBytes !== undefined
            ? Number((currentRow.v2rayQuotaBytes / BYTES_PER_GB).toFixed(2))
            : undefined,
        v2rayMaxPackages: currentRow.v2rayMaxPackages ?? undefined,
        canResellDns: currentRow.canResellDns ?? false,
        dnsQuotaBytes:
          currentRow.dnsQuotaBytes !== null &&
          currentRow.dnsQuotaBytes !== undefined
            ? Number((currentRow.dnsQuotaBytes / BYTES_PER_GB).toFixed(2))
            : undefined,
        dnsMaxAccounts: currentRow.dnsMaxAccounts ?? undefined,
        canCreateApplications: currentRow.canCreateApplications ?? false,
        applicationQuotaBytes:
          currentRow.applicationQuotaBytes !== null &&
          currentRow.applicationQuotaBytes !== undefined
            ? Number(
                (currentRow.applicationQuotaBytes / BYTES_PER_GB).toFixed(2)
              )
            : undefined,
        applicationMaxCount: currentRow.applicationMaxCount ?? undefined,
        billingMode: currentRow.billingMode ?? 'VOLUME',
        paymentSubMode: currentRow.paymentSubMode ?? 'PREPAID',
        debtLimitAmount:
          currentRow.debtLimitAmount !== null &&
          currentRow.debtLimitAmount !== undefined
            ? currentRow.debtLimitAmount
            : undefined,
      })
    }
  }, [currentRow, form])

  useEffect(() => {
    setPending(
      isEdit
        ? isUpdatePending ||
            isAssignPending ||
            isAssignGroupsPending ||
            isAssignProfilesPending ||
            isAssignPanelsPending ||
            isAssignDNSPanelsPending
        : isCreatePending ||
            isAssignPending ||
            isAssignGroupsPending ||
            isAssignProfilesPending ||
            isAssignPanelsPending ||
            isAssignDNSPanelsPending
    )
  }, [
    isCreatePending,
    isEdit,
    isUpdatePending,
    isAssignPending,
    isAssignGroupsPending,
    isAssignProfilesPending,
    isAssignPanelsPending,
    isAssignDNSPanelsPending,
    setPending,
  ])

  const onSubmit = async (
    values: CreateResellerRequest | UpdateResellerRequest
  ) => {
    try {
      const quotaGbRaw = (values as any).quotaBytes
      const maxPeersRaw = (values as any).maxPeers
      const maxPeersCleared =
        maxPeersRaw === '' || maxPeersRaw === null || maxPeersRaw === undefined
      const userManagerQuotaGbRaw = (values as any).userManagerQuotaBytes
      const userManagerMaxAccountsRaw = (values as any).userManagerMaxAccounts
      const userManagerMaxAccountsCleared =
        userManagerMaxAccountsRaw === '' ||
        userManagerMaxAccountsRaw === null ||
        userManagerMaxAccountsRaw === undefined
      const v2rayQuotaGbRaw = (values as any).v2rayQuotaBytes
      const v2rayMaxPackagesRaw = (values as any).v2rayMaxPackages
      const v2rayMaxPackagesCleared =
        v2rayMaxPackagesRaw === '' ||
        v2rayMaxPackagesRaw === null ||
        v2rayMaxPackagesRaw === undefined
      const dnsQuotaGbRaw = (values as any).dnsQuotaBytes
      const dnsMaxAccountsRaw = (values as any).dnsMaxAccounts
      const dnsMaxAccountsCleared =
        dnsMaxAccountsRaw === '' ||
        dnsMaxAccountsRaw === null ||
        dnsMaxAccountsRaw === undefined
      const applicationQuotaGbRaw = (values as any).applicationQuotaBytes
      const applicationMaxCountRaw = (values as any).applicationMaxCount
      const applicationMaxCountCleared =
        applicationMaxCountRaw === '' ||
        applicationMaxCountRaw === null ||
        applicationMaxCountRaw === undefined
      const debtLimitAmountRaw = (values as any).debtLimitAmount
      const debtLimitAmountCleared =
        debtLimitAmountRaw === '' ||
        debtLimitAmountRaw === null ||
        debtLimitAmountRaw === undefined
      const payload = {
        ...values,
        quotaBytes:
          quotaGbRaw === '' || quotaGbRaw === null || quotaGbRaw === undefined
            ? undefined
            : Math.round(Number(quotaGbRaw) * BYTES_PER_GB),
        maxPeers: maxPeersCleared ? undefined : Number(maxPeersRaw),
        ...(isEdit ? { clearMaxPeers: maxPeersCleared } : {}),
        userManagerQuotaBytes:
          userManagerQuotaGbRaw === '' ||
          userManagerQuotaGbRaw === null ||
          userManagerQuotaGbRaw === undefined
            ? undefined
            : Math.round(Number(userManagerQuotaGbRaw) * BYTES_PER_GB),
        userManagerMaxAccounts: userManagerMaxAccountsCleared
          ? undefined
          : Number(userManagerMaxAccountsRaw),
        ...(isEdit
          ? { clearUserManagerMaxAccounts: userManagerMaxAccountsCleared }
          : {}),
        v2rayQuotaBytes:
          v2rayQuotaGbRaw === '' ||
          v2rayQuotaGbRaw === null ||
          v2rayQuotaGbRaw === undefined
            ? undefined
            : Math.round(Number(v2rayQuotaGbRaw) * BYTES_PER_GB),
        v2rayMaxPackages: v2rayMaxPackagesCleared
          ? undefined
          : Number(v2rayMaxPackagesRaw),
        ...(isEdit ? { clearV2RayMaxPackages: v2rayMaxPackagesCleared } : {}),
        dnsQuotaBytes:
          dnsQuotaGbRaw === '' ||
          dnsQuotaGbRaw === null ||
          dnsQuotaGbRaw === undefined
            ? undefined
            : Math.round(Number(dnsQuotaGbRaw) * BYTES_PER_GB),
        dnsMaxAccounts: dnsMaxAccountsCleared
          ? undefined
          : Number(dnsMaxAccountsRaw),
        ...(isEdit ? { clearDnsMaxAccounts: dnsMaxAccountsCleared } : {}),
        applicationQuotaBytes:
          applicationQuotaGbRaw === '' ||
          applicationQuotaGbRaw === null ||
          applicationQuotaGbRaw === undefined
            ? undefined
            : Math.round(Number(applicationQuotaGbRaw) * BYTES_PER_GB),
        applicationMaxCount: applicationMaxCountCleared
          ? undefined
          : Number(applicationMaxCountRaw),
        ...(isEdit
          ? { clearApplicationMaxCount: applicationMaxCountCleared }
          : {}),
        debtLimitAmount: debtLimitAmountCleared
          ? undefined
          : Number(debtLimitAmountRaw),
        ...(isEdit ? { clearDebtLimitAmount: debtLimitAmountCleared } : {}),
      } as CreateResellerRequest | UpdateResellerRequest

      if (isEdit && (payload as UpdateResellerRequest).password === '') {
        delete (payload as any).password
      }

      const billingModeValue = (values as any).billingMode
      const buildBillingPricesPayload = () => {
        const prices: {
          product: string
          locationKey: string
          pricePerGbAmount: number
        }[] = []
        for (const product of [
          'WIREGUARD',
          'USER_MANAGER',
          'V2RAY',
          'APPLICATION',
        ]) {
          const raw = billingPriceAmounts[product]
          if (raw !== undefined && raw !== '') {
            prices.push({
              product,
              locationKey: '',
              pricePerGbAmount: Math.round(Number(raw)),
            })
          }
        }
        return prices
      }

      if (isEdit && currentRow) {
        await updateReseller({
          ...payload,
          id: currentRow.id,
        } as UpdateResellerRequest)
        await setAssignedInterfaces({
          resellerId: currentRow.id as number,
          interfaceIds: selectedInterfaceIds,
        })
        await setAssignedUserManagerGroups({
          resellerId: currentRow.id as number,
          groupNames: selectedUmGroups,
        })
        await setAssignedUserManagerProfiles({
          resellerId: currentRow.id as number,
          profileNames: selectedUmProfiles,
        })
        await setAssignedXuiPanels({
          resellerId: currentRow.id as number,
          panelIds: selectedPanelIds,
        })
        await setAssignedDNSPanels({
          resellerId: currentRow.id as number,
          panelIds: selectedDNSPanelIds,
        })
        if (billingModeValue === 'PAYMENT') {
          await setResellerBillingPrices({
            resellerId: currentRow.id as number,
            prices: buildBillingPricesPayload(),
          })
        }
        toast.success('نماینده با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        // The reseller row itself is created FIRST and independently from
        // every assignment call below -- a confirmed, reported bug: if any
        // assignment call (interfaces/UM groups/UM profiles/xui panels)
        // failed AFTER a successful create, the whole flow threw into the
        // outer catch below, which showed a generic failure toast and left
        // the dialog open with the form still filled in. The admin,
        // reasonably assuming nothing had happened, resubmitted with the
        // same username -- which the ALREADY-CREATED reseller from the
        // first attempt then rejected as "username already exists" (the
        // exact reported symptom), while the successfully-created reseller
        // itself was sitting correctly in the list the whole time (easy to
        // miss if the admin never scrolled/refreshed, reading as "no
        // reseller shows up" even though one actually had been created).
        // Assignment failures are now caught separately: the create's own
        // success is reported and the dialog closes regardless, with a
        // distinct warning naming exactly which assignment step failed
        // (the admin can re-open Edit on the newly-created reseller to
        // retry just that one assignment, instead of the whole reseller).
        const created = await createReseller(payload as CreateResellerRequest)

        const assignmentFailures: string[] = []
        const runAssignment = async (label: string, fn: () => Promise<unknown>) => {
          try {
            await fn()
          } catch (assignError) {
            assignmentFailures.push(
              `${label}: ${getApiErrorMessage(assignError, 'خطای نامشخص')}`
            )
          }
        }

        if (selectedInterfaceIds.length > 0) {
          await runAssignment('اینترفیس‌ها', () =>
            setAssignedInterfaces({
              resellerId: created.id,
              interfaceIds: selectedInterfaceIds,
            })
          )
        }
        if (selectedUmGroups.length > 0) {
          await runAssignment('گروه‌های یوزر منیجر', () =>
            setAssignedUserManagerGroups({
              resellerId: created.id,
              groupNames: selectedUmGroups,
            })
          )
        }
        if (selectedUmProfiles.length > 0) {
          await runAssignment('پروفایل‌های یوزر منیجر', () =>
            setAssignedUserManagerProfiles({
              resellerId: created.id,
              profileNames: selectedUmProfiles,
            })
          )
        }
        if (selectedPanelIds.length > 0) {
          await runAssignment('پنل‌های V2Ray', () =>
            setAssignedXuiPanels({
              resellerId: created.id,
              panelIds: selectedPanelIds,
            })
          )
        }
        if (selectedDNSPanelIds.length > 0) {
          await runAssignment('پنل‌های DNS', () =>
            setAssignedDNSPanels({
              resellerId: created.id,
              panelIds: selectedDNSPanelIds,
            })
          )
        }
        if (billingModeValue === 'PAYMENT') {
          await runAssignment('قیمت‌های صورتحساب', () =>
            setResellerBillingPrices({
              resellerId: created.id,
              prices: buildBillingPricesPayload(),
            })
          )
        }

        if (assignmentFailures.length > 0) {
          toast.error(
            `نماینده ایجاد شد، اما برخی تخصیص‌ها ناموفق بودند -- برای تلاش مجدد نماینده را ویرایش کنید: ${assignmentFailures.join('; ')}`,
            { duration: 8000 }
          )
        } else {
          toast.success('نماینده با موفقیت ایجاد شد.', { duration: 5000 })
        }
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره نماینده ناموفق بود. لطفاً دوباره تلاش کنید.')
      )
    }
  }

  return (
    <Form {...form}>
      <form
        id='reseller-form'
        onSubmit={form.handleSubmit(onSubmit)}
        className='space-y-4'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <FormField
            control={form.control}
            name='name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>نام</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='username'
            render={({ field }) => (
              <FormItem>
                <FormLabel>نام کاربری</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='email'
            render={({ field }) => (
              <FormItem>
                <FormLabel>ایمیل</FormLabel>
                <FormControl>
                  <Input {...field} type='email' />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='password'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{isEdit ? 'رمز عبور جدید (اختیاری)' : 'رمز عبور'}</FormLabel>
                <FormControl>
                  <PasswordInput
                    value={field.value ?? ''}
                    onChange={field.onChange}
                    placeholder={isEdit ? 'برای حفظ رمز عبور فعلی خالی بگذارید' : 'رمز عبور را وارد کنید'}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='quotaBytes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>سهمیه (گیگابایت)</FormLabel>
                <FormControl>
                  <Input type='number' step='0.01' min='0' {...field} />
                </FormControl>
                <FormDescription>
                  برای سهمیه نامحدود خالی بگذارید. مقدار به‌صورت خودکار به بایت تبدیل می‌شود.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='maxPeers'
            render={({ field }) => (
              <FormItem>
                <FormLabel>سقف تعداد کاربران</FormLabel>
                <FormControl>
                  <Input type='number' step='1' min='1' {...field} />
                </FormControl>
                <FormDescription>
                  حداکثر تعداد وایرگاردهایی که این نماینده می‌تواند ایجاد کند.
                  برای نامحدود خالی بگذارید.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='telegramChatId'
            render={({ field }) => (
              <FormItem>
                <FormLabel>شناسه چت تلگرام (اختیاری)</FormLabel>
                <FormControl>
                  <Input {...field} value={field.value ?? ''} />
                </FormControl>
                <FormDescription>
                  به ربات اجازه می‌دهد اعلان‌های اختصاصی این نماینده (هشدار
                  ورود، هشدار سهمیه و غیره) را ارسال کند. برای یافتن chat_id
                  به @userinfobot در تلگرام پیام دهید.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='otpEnabled'
            render={({ field }) => (
              <FormItem className='flex items-center gap-4 rounded-lg border p-4'>
                <div>
                  <FormLabel>ورود دو مرحله‌ای (OTP)</FormLabel>
                  <FormDescription>
                    کد تایید ۴ رقمی از طریق ربات تلگرام به شناسه چت بالا
                    ارسال می‌شود. بدون تنظیم شناسه چت تلگرام، این گزینه
                    نادیده گرفته می‌شود.
                  </FormDescription>
                </div>
                <FormControl>
                  <Checkbox
                    checked={Boolean(field.value)}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />
          {isEdit && (
            <FormField
              control={form.control}
              name='isActive'
              render={({ field }) => (
                <FormItem className='flex items-center gap-4 border p-4 rounded-lg'>
                  <div>
                    <FormLabel>فعال</FormLabel>
                    <FormDescription>
                      فعال یا غیرفعال کردن این نماینده.
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Checkbox
                      checked={Boolean(field.value)}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          )}
        </div>

        <div className='space-y-2 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>اینترفیس‌های تخصیص‌یافته (اختیاری)</p>
            <p className='text-muted-foreground text-sm'>
              این نماینده فقط می‌تواند روی اینترفیس‌های انتخاب‌شده در زیر
              وایرگارد ایجاد کند. برای نماینده‌ای که فقط باید از
              حساب‌های یوزر منیجر (L2TP/PPTP/SSTP/OpenVPN) در پایین استفاده
              کند، همه را بدون تیک بگذارید -- دسترسی WireGuard و دسترسی
              یوزر منیجر مستقل از هم هستند و هیچ‌کدام به دیگری نیاز ندارد.
            </p>
          </div>
          <div className='grid gap-2 md:grid-cols-2'>
            {interfacesList.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                هیچ اینترفیسی موجود نیست.
              </p>
            ) : (
              interfacesList.map((iface) => (
                <label
                  key={iface.id}
                  className='flex items-center gap-2 text-sm'
                >
                  <Checkbox
                    checked={selectedInterfaceIds.includes(iface.id)}
                    onCheckedChange={(checked) =>
                      toggleInterface(iface.id, Boolean(checked))
                    }
                  />
                  {iface.name}
                </label>
              ))
            )}
          </div>
        </div>

        <div className='space-y-4 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>یوزر منیجر</p>
            <p className='text-muted-foreground text-sm'>
              دسترسی این نماینده به ایجاد حساب L2TP/PPTP/SSTP/OpenVPN را
              کنترل می‌کند. این سهمیه و سقف تعداد حساب کاملاً مستقل از
              تنظیمات سهمیه/سقف کاربران WireGuard در بالاست.
            </p>
          </div>

          <FormField
            control={form.control}
            name='canCreateUserManagerAccounts'
            render={({ field }) => (
              <FormItem className='flex items-center gap-4'>
                <FormControl>
                  <Checkbox
                    checked={Boolean(field.value)}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div>
                  <FormLabel>امکان ایجاد حساب‌های یوزر منیجر</FormLabel>
                  <FormDescription>
                    به این نماینده اجازه می‌دهد حساب‌های L2TP/PPTP/SSTP/OpenVPN
                    ایجاد کند.
                  </FormDescription>
                </div>
              </FormItem>
            )}
          />

          <div className='grid gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='userManagerQuotaBytes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>سهمیه یوزر منیجر (گیگابایت)</FormLabel>
                  <FormControl>
                    <Input type='number' step='0.01' min='0' {...field} />
                  </FormControl>
                  <FormDescription>
                    برای نامحدود خالی بگذارید. مستقل از سهمیه WireGuard در
                    بالاست.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='userManagerMaxAccounts'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>سقف حساب‌های یوزر منیجر</FormLabel>
                  <FormControl>
                    <Input type='number' step='1' min='1' {...field} />
                  </FormControl>
                  <FormDescription>
                    حداکثر تعداد حساب‌های L2TP/PPTP/SSTP/OpenVPN که این
                    نماینده می‌تواند ایجاد کند. برای نامحدود خالی بگذارید.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          {canCreateUserManagerAccounts && (
            <>
              <div className='space-y-2'>
                <p className='text-sm font-medium'>گروه‌های مجاز</p>
                <p className='text-muted-foreground text-sm'>
                  این نماینده هنگام ایجاد حساب فقط می‌تواند گروه‌های یوزر
                  منیجر انتخاب‌شده در زیر را تخصیص دهد. اگر همه را بدون تیک
                  بگذارید، نماینده نمی‌تواند هیچ حسابی ایجاد کند.
                </p>
                <div className='grid gap-2 md:grid-cols-2'>
                  {umGroupsList.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>
                      هیچ گروهی روی روتر یافت نشد.
                    </p>
                  ) : (
                    umGroupsList.map((group) => (
                      <label
                        key={group.name}
                        className='flex items-center gap-2 text-sm'
                      >
                        <Checkbox
                          checked={selectedUmGroups.includes(group.name)}
                          onCheckedChange={(checked) =>
                            toggleUmGroup(group.name, Boolean(checked))
                          }
                        />
                        {group.name}
                      </label>
                    ))
                  )}
                </div>
              </div>

              <div className='space-y-2'>
                <p className='text-sm font-medium'>پروفایل‌های مجاز</p>
                <p className='text-muted-foreground text-sm'>
                  این نماینده هنگام ساخت حساب فقط می‌تواند از پروفایل‌های
                  User Manager انتخاب‌شده‌ی زیر استفاده کند.
                </p>
                <div className='grid gap-2 md:grid-cols-2'>
                  {umProfilesList.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>
                      No profiles found on the router.
                    </p>
                  ) : (
                    umProfilesList.map((profile) => (
                      <label
                        key={profile.name}
                        className='flex items-center gap-2 text-sm'
                      >
                        <Checkbox
                          checked={selectedUmProfiles.includes(profile.name)}
                          onCheckedChange={(checked) =>
                            toggleUmProfile(profile.name, Boolean(checked))
                          }
                        />
                        {profile.name}
                      </label>
                    ))
                  )}
                </div>
              </div>
            </>
          )}
        </div>

        <div className='space-y-4 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>V2Ray</p>
            <p className='text-muted-foreground text-sm'>
              Controls this reseller&apos;s access to V2Ray package creation
              across the registered x-ui panels. This quota and package
              limit are completely separate from the WireGuard and User
              Manager settings above.
            </p>
          </div>

          <FormField
            control={form.control}
            name='canResellV2Ray'
            render={({ field }) => (
              <FormItem className='flex items-center gap-4'>
                <FormControl>
                  <Checkbox
                    checked={Boolean(field.value)}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div>
                  <FormLabel>اجازه‌ی فروش V2Ray</FormLabel>
                  <FormDescription>
                    به این نماینده اجازه می‌دهد پکیج‌های V2Ray بسازد.
                  </FormDescription>
                </div>
              </FormItem>
            )}
          />

          <div className='grid gap-4 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='v2rayQuotaBytes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>سقف حجم V2Ray (گیگابایت)</FormLabel>
                  <FormControl>
                    <Input type='number' step='0.01' min='0' {...field} />
                  </FormControl>
                  <FormDescription>
                    خالی بگذارید برای نامحدود. مستقل از سقف حجم وایرگارد و
                    یوزرمنیجر بالا.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='v2rayMaxPackages'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>حداکثر تعداد پکیج V2Ray</FormLabel>
                  <FormControl>
                    <Input type='number' step='1' min='1' {...field} />
                  </FormControl>
                  <FormDescription>
                    حداکثر تعداد پکیج V2Ray که این نماینده می‌تواند بسازد.
                    خالی بگذارید برای نامحدود.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          {canResellV2Ray && (
            <div className='space-y-2'>
              <p className='text-sm font-medium'>سرورهای X-UI مجاز</p>
              <p className='text-muted-foreground text-sm'>
                این نماینده فقط می‌تواند روی پنل‌های x-ui انتخاب‌شده در زیر
                پکیج V2Ray بسازد. همه را بدون تیک بگذارید تا این نماینده
                حتی با دسترسی V2Ray فعال بالا، نتواند هیچ پکیجی بسازد.
              </p>
              <div className='grid gap-2 md:grid-cols-2'>
                {xuiPanelsList.length === 0 ? (
                  <p className='text-muted-foreground text-sm'>
                    هنوز هیچ پنل x-ui ثبت نشده است. ابتدا یکی را در بخش
                    سرورهای X-UI اضافه کنید.
                  </p>
                ) : (
                  xuiPanelsList.map((panel) => (
                    <label
                      key={panel.id}
                      className='flex items-center gap-2 text-sm'
                    >
                      <Checkbox
                        checked={selectedPanelIds.includes(panel.id)}
                        onCheckedChange={(checked) =>
                          toggleXuiPanel(panel.id, Boolean(checked))
                        }
                      />
                      {panel.name}
                    </label>
                  ))
                )}
              </div>
            </div>
          )}
        </div>

        <div className='space-y-4 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>DNS</p>
            <p className='text-muted-foreground text-sm'>
              کنترل دسترسی این نماینده به ساخت حساب Smart DNS. این سقف حجم و
              حداکثر تعداد حساب کاملاً مستقل از وایرگارد، یوزرمنیجر و V2Ray
              بالاست.
            </p>
          </div>

          <FormField
            control={form.control}
            name='canResellDns'
            render={({ field }) => (
              <FormItem className='flex items-center gap-4'>
                <FormControl>
                  <Checkbox
                    checked={Boolean(field.value)}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div>
                  <FormLabel>اجازه‌ی فروش DNS</FormLabel>
                  <FormDescription>
                    به این نماینده اجازه می‌دهد حساب‌های Smart DNS بسازد.
                  </FormDescription>
                </div>
              </FormItem>
            )}
          />

          {canResellDns && (
            <>
              <div className='grid gap-4 md:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='dnsQuotaBytes'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>سقف حجم DNS (گیگابایت)</FormLabel>
                      <FormControl>
                        <Input type='number' step='0.01' min='0' {...field} />
                      </FormControl>
                      <FormDescription>
                        خالی بگذارید برای نامحدود. مستقل از سقف حجم
                        پروتکل‌های دیگر بالا.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='dnsMaxAccounts'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>حداکثر تعداد حساب DNS</FormLabel>
                      <FormControl>
                        <Input type='number' step='1' min='1' {...field} />
                      </FormControl>
                      <FormDescription>
                        حداکثر تعداد حساب DNS که این نماینده می‌تواند بسازد.
                        خالی بگذارید برای نامحدود.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='space-y-2'>
                <p className='text-muted-foreground text-sm'>
                  این نماینده فقط می‌تواند حساب‌های Smart DNS را روی پنل‌های
                  DNS انتخاب‌شده در زیر ایجاد کند. همه را بدون تیک بگذارید تا
                  این نماینده حتی با دسترسی DNS فعال بالا، نتواند هیچ حسابی
                  بسازد.
                </p>
                <div className='grid gap-2 md:grid-cols-2'>
                  {dnsPanelsList.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>
                      هنوز هیچ پنل DNS ثبت نشده است. ابتدا یکی را در بخش
                      پنل‌های DNS اضافه کنید.
                    </p>
                  ) : (
                    dnsPanelsList.map((panel) => (
                      <label
                        key={panel.id}
                        className='flex items-center gap-2 text-sm'
                      >
                        <Checkbox
                          checked={selectedDNSPanelIds.includes(panel.id)}
                          onCheckedChange={(checked) =>
                            toggleDNSPanel(panel.id, Boolean(checked))
                          }
                        />
                        {panel.name}
                      </label>
                    ))
                  )}
                </div>
              </div>
            </>
          )}
        </div>

        <div className='space-y-4 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>اپلیکیشن‌ها</p>
            <p className='text-muted-foreground text-sm'>
              اجازه‌ی ساخت باندل «اپلیکیشن» (شناسه‌ی ورود موبایل) برای این
              نماینده. این سقف حجم و حداکثر تعداد اپلیکیشن کاملاً مستقل از
              وایرگارد، یوزرمنیجر، V2Ray و DNS بالاست -- هر اپلیکیشن همچنان
              از منابع پروتکلی که نماینده به آن‌ها دسترسی دارد استفاده
              می‌کند و سقف حجمی همان‌ها هم جداگانه اعمال می‌شود.
            </p>
          </div>

          <FormField
            control={form.control}
            name='canCreateApplications'
            render={({ field }) => (
              <FormItem className='flex items-center gap-4'>
                <FormControl>
                  <Checkbox
                    checked={Boolean(field.value)}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
                <div>
                  <FormLabel>اجازه‌ی ساخت اپلیکیشن</FormLabel>
                  <FormDescription>
                    به این نماینده اجازه می‌دهد باندل اپلیکیشن بسازد.
                  </FormDescription>
                </div>
              </FormItem>
            )}
          />

          {canCreateApplications && (
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='applicationQuotaBytes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>سقف حجم اپلیکیشن (گیگابایت)</FormLabel>
                    <FormControl>
                      <Input type='number' step='0.01' min='0' {...field} />
                    </FormControl>
                    <FormDescription>
                      خالی بگذارید برای نامحدود. مستقل از سقف حجم پروتکل‌های
                      دیگر بالا.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='applicationMaxCount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>حداکثر تعداد اپلیکیشن</FormLabel>
                    <FormControl>
                      <Input type='number' step='1' min='1' {...field} />
                    </FormControl>
                    <FormDescription>
                      خالی بگذارید برای نامحدود.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          )}
        </div>

        <div className='space-y-4 rounded-lg border p-4'>
          <div>
            <p className='text-sm font-medium'>حالت صورتحساب</p>
            <p className='text-muted-foreground text-sm'>
              حالت حجمی (پیش‌فرض) سقف‌های ثابت تعیین‌شده در بالا را برای هر
              محصول اعمال می‌کند. حالت پرداختی به‌جای آن، به ازای هر گیگابایت
              مصرف در همه‌ی محصولات، از کیف پول این نماینده کسر می‌کند، مستقل
              از فیلدهای سقف حجم بالا.
            </p>
          </div>

          {isEdit &&
            currentRow?.billingMode &&
            billingMode &&
            currentRow.billingMode !== billingMode && (
              <div className='rounded-md border border-amber-500/50 bg-amber-500/10 p-3 text-sm text-amber-700 dark:text-amber-400'>
                {billingMode === 'PAYMENT' ? (
                  <>
                    با ذخیره‌ی این تغییر، این نماینده از حالت حجمی به پرداختی
                    سوییچ می‌کند: سقف‌های حجم بالا دیگر باعث قطع سرویس نمی‌شوند
                    (فقط موجودی کیف پول تعیین‌کننده است)، هر منبعی که به‌خاطر
                    اتمام حجم قبلاً غیرفعال شده بود دوباره فعال می‌شود، و در
                    صورت نبود کیف پول، یکی با موجودی صفر ساخته خواهد شد.
                  </>
                ) : (
                  <>
                    با ذخیره‌ی این تغییر، این نماینده از حالت پرداختی به حجمی
                    سوییچ می‌کند: از این پس فقط سقف‌های حجم بالا اعمال می‌شوند
                    (نه موجودی کیف پول)، و هر منبعی که به‌خاطر کمبود موجودی
                    قبلاً غیرفعال شده بود دوباره فعال می‌شود.
                  </>
                )}
              </div>
            )}

          <FormField
            control={form.control}
            name='billingMode'
            render={({ field }) => (
              <FormItem>
                <FormLabel>حالت صورتحساب</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='VOLUME'>حجمی</SelectItem>
                    <SelectItem value='PAYMENT'>پرداختی</SelectItem>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          {billingMode === 'PAYMENT' && (
            <>
              <FormField
                control={form.control}
                name='paymentSubMode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>زیرحالت پرداخت</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='PREPAID'>
                          پیش‌پرداخت -- با اتمام موجودی غیرفعال می‌شود
                        </SelectItem>
                        <SelectItem value='POSTPAID'>
                          پس‌پرداخت -- می‌تواند تا سقفی بدهکار شود
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {paymentSubMode === 'POSTPAID' && (
                <FormField
                  control={form.control}
                  name='debtLimitAmount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>سقف بدهی (تومان)</FormLabel>
                      <FormControl>
                        <Input type='number' step='1' min='0' {...field} />
                      </FormControl>
                      <FormDescription>
                        حداکثر موجودی منفی که این نماینده می‌تواند تا
                        غیرفعال‌شدن داشته باشد. خالی بگذارید برای بدهی
                        نامحدود.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <div className='space-y-3 rounded-md border p-3'>
                <div>
                  <p className='text-sm font-medium'>
                    قیمت هر گیگابایت (تومان)
                  </p>
                  <p className='text-muted-foreground text-sm'>
                    هر گیگابایت مصرف این محصول، این مبلغ را از کیف پول
                    نماینده کسر می‌کند. یک محصول را خالی بگذارید تا فقط
                    مصرفش ثبت شود بدون شارژ. این قیمت ثابت و کلی محصول است --
                    برای قیمت‌گذاری بر اساس لوکیشن (مثلاً اینترفیس‌های
                    اروپایی ارزان‌تر وایرگارد) یا سطوح تخفیف حجمی، از صفحه‌ی
                    قیمت‌های صورتحساب خود این نماینده استفاده کنید
                    {isEdit && currentRow ? ' (پس از ذخیره)' : ''}.
                  </p>
                </div>
                <div className='grid gap-3 sm:grid-cols-3'>
                  {(
                    [
                      { key: 'WIREGUARD', label: 'WireGuard' },
                      { key: 'USER_MANAGER', label: 'User Manager' },
                      { key: 'V2RAY', label: 'V2Ray' },
                      { key: 'APPLICATION', label: 'اپلیکیشن‌ها' },
                    ] as const
                  ).map((product) => (
                    <div key={product.key} className='space-y-1'>
                      <Label htmlFor={`billing-price-${product.key}`}>
                        {product.label}
                      </Label>
                      <Input
                        id={`billing-price-${product.key}`}
                        type='number'
                        min='0'
                        step='1'
                        placeholder='0'
                        value={billingPriceAmounts[product.key] ?? ''}
                        onChange={(e) =>
                          setBillingPriceAmounts((prev) => ({
                            ...prev,
                            [product.key]: e.target.value,
                          }))
                        }
                      />
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>

        {isEdit && currentRow && (
          <div className='space-y-3 rounded-lg border p-4'>
            <div>
              <p className='text-sm font-medium'>Wallet Top-up</p>
              <p className='text-muted-foreground text-sm'>
                Credit this reseller's wallet directly -- use this once
                they've paid you outside the panel (bank transfer, cash,
                etc).
              </p>
            </div>
            <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
              <div className='flex-1 space-y-1'>
                <Label htmlFor='wallet-top-up-amount'>Amount (Toman)</Label>
                <Input
                  id='wallet-top-up-amount'
                  type='number'
                  min='1'
                  step='1'
                  placeholder='0'
                  value={walletTopUpAmount}
                  onChange={(e) => setWalletTopUpAmount(e.target.value)}
                />
              </div>
              <Button
                type='button'
                disabled={
                  isWalletCreditPending ||
                  !walletTopUpAmount ||
                  Number(walletTopUpAmount) <= 0
                }
                onClick={() =>
                  creditWalletMutate(Math.round(Number(walletTopUpAmount)))
                }
              >
                {isWalletCreditPending ? 'Crediting...' : 'Credit Wallet'}
              </Button>
            </div>
          </div>
        )}
      </form>
    </Form>
  )
}
