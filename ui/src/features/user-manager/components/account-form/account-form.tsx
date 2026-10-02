'use client'

import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  CreateUserManagerAccountRequest,
  CreateUserManagerAccountSchema,
  UpdateUserManagerAccountRequest,
  UpdateUserManagerAccountSchema,
  USER_MANAGER_PROTOCOLS,
  UserManagerAccount,
} from '@/schema/user-manager.ts'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
import { useAssignedUserManagerGroupsQuery } from '@/hooks/resellers/useAssignedUserManagerGroupsQuery.ts'
import { useAssignedUserManagerProfilesQuery } from '@/hooks/resellers/useAssignedUserManagerProfilesQuery.ts'
import { useCreateUserManagerAccountMutation } from '@/hooks/user-manager/useCreateUserManagerAccountMutation.ts'
import { useUpdateUserManagerAccountMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountMutation.ts'
import { useCreateUserManagerAccountForResellerMutation } from '@/hooks/user-manager/useCreateUserManagerAccountForResellerMutation.ts'
import { useUpdateUserManagerAccountForResellerMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountForResellerMutation.ts'
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
import { PasswordInput } from '@/components/password-input.tsx'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'
import { SimpleDatepicker } from '@/features/shared-components/simple-date-picker.tsx'

interface Props {
  currentRow?: UserManagerAccount
  onClose: () => void
  setIsLoading?: (loading: boolean) => void
  // When set, an admin is creating/editing this account on behalf of a
  // specific reseller (admin "Reseller User Manager" page).
  targetResellerId?: number
  formId?: string
}

export function AccountForm({
  currentRow,
  onClose,
  setIsLoading,
  targetResellerId,
  formId = 'user-manager-account-form',
}: Props) {
  const isEdit = !!currentRow
  const isForReseller = targetResellerId !== undefined

  const admin = useAuthStore((state) => state.auth.admin)
  const isSelfServeReseller = admin?.role === 'reseller'
  // The reseller whose group/profile assignment restrictions apply here:
  // the admin-on-behalf-of target reseller, or the logged-in reseller
  // managing their own accounts. Neither applies for the admin's own
  // account list, which sees every group/profile unfiltered.
  const restrictingResellerId = isForReseller
    ? targetResellerId
    : isSelfServeReseller
      ? (admin?.reseller_id ?? undefined)
      : undefined

  const { data: groups = [], isLoading: isGroupsLoading } =
    useUserManagerGroupsQuery()
  const { data: profiles = [], isLoading: isProfilesLoading } =
    useUserManagerProfilesQuery()
  const { data: assignedGroupNames } = useAssignedUserManagerGroupsQuery(
    restrictingResellerId
  )
  const { data: assignedProfileNames } = useAssignedUserManagerProfilesQuery(
    restrictingResellerId
  )

  // When a restriction applies, only show the groups/profiles the admin
  // has explicitly assigned to this reseller -- matches the backend's
  // "must be explicitly assigned" enforcement exactly (see
  // ensureResellerCanUseGroup/ensureResellerCanUseProfile), so the
  // dropdown never lets a reseller pick something the server would reject.
  const visibleGroups =
    restrictingResellerId !== undefined && assignedGroupNames
      ? groups.filter((g) => assignedGroupNames.includes(g.name))
      : groups
  const visibleProfiles =
    restrictingResellerId !== undefined && assignedProfileNames
      ? profiles.filter((p) => assignedProfileNames.includes(p.name))
      : profiles

  const form = useForm<
    CreateUserManagerAccountRequest | UpdateUserManagerAccountRequest
  >({
    resolver: zodResolver(
      (isEdit
        ? UpdateUserManagerAccountSchema
        : CreateUserManagerAccountSchema) as never
    ),
    defaultValues: isEdit
      ? {
          id: currentRow.id,
          group: currentRow.group,
          profile: currentRow.profile,
          protocols: currentRow.protocols,
          comment: currentRow.comment ?? '',
          shared_users: currentRow.shared_users,
          traffic_limit: currentRow.traffic_limit ?? '',
          expire_time: currentRow.expire_time ?? '',
        }
      : {
          username: '',
          password: '',
          group: '',
          profile: '',
          protocols: ['l2tp'],
          shared_users: 1,
          comment: '',
          traffic_limit: '',
          expire_time: '',
        },
  })

  const { mutateAsync: createAccount, isPending: isCreatePending } =
    useCreateUserManagerAccountMutation()
  const { mutateAsync: updateAccount, isPending: isUpdatePending } =
    useUpdateUserManagerAccountMutation()
  const {
    mutateAsync: createAccountForReseller,
    isPending: isCreateForResellerPending,
  } = useCreateUserManagerAccountForResellerMutation()
  const {
    mutateAsync: updateAccountForReseller,
    isPending: isUpdateForResellerPending,
  } = useUpdateUserManagerAccountForResellerMutation()

  const isPending = isForReseller
    ? isEdit
      ? isUpdateForResellerPending
      : isCreateForResellerPending
    : isEdit
      ? isUpdatePending
      : isCreatePending

  useEffect(() => {
    setIsLoading?.(isPending)
  }, [isPending, setIsLoading])

  const onSubmit = async (
    values: CreateUserManagerAccountRequest | UpdateUserManagerAccountRequest
  ) => {
    // A confirmed, reported bug (same root cause found and fixed in
    // v2ray-form.tsx/xui-panel-form.tsx): this block previously had no
    // try/catch, so any mutation failure was an unhandled rejection that
    // crashed the whole page to the app-wide "500 -- Oops! Something went
    // wrong" error boundary instead of showing a normal toast.
    try {
      if (isForReseller) {
        if (isEdit) {
          await updateAccountForReseller({
            resellerId: targetResellerId,
            account: values as UpdateUserManagerAccountRequest,
          })
          toast.success('حساب با موفقیت به‌روزرسانی شد.', { duration: 5000 })
        } else {
          await createAccountForReseller({
            resellerId: targetResellerId,
            account: values as CreateUserManagerAccountRequest,
          })
          toast.success('حساب با موفقیت ایجاد شد.', { duration: 5000 })
        }
      } else if (isEdit) {
        await updateAccount(values as UpdateUserManagerAccountRequest)
        toast.success('حساب با موفقیت به‌روزرسانی شد.', { duration: 5000 })
      } else {
        await createAccount(values as CreateUserManagerAccountRequest)
        toast.success('حساب با موفقیت ایجاد شد.', { duration: 5000 })
      }
      form.reset()
      onClose()
    } catch (error) {
      toast.error(
        getApiErrorMessage(error, 'ذخیره حساب ناموفق بود. دوباره تلاش کنید.')
      )
    }
  }

  return (
    <Form {...form}>
      <form
        id={formId}
        onSubmit={form.handleSubmit(onSubmit)}
        className='space-y-4'
      >
        <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
          {!isEdit && (
            <>
              <FormField
                control={form.control}
                name='username'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>نام کاربری</FormLabel>
                    <FormControl>
                      <Input placeholder='نام کاربری' {...field} />
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
                    <FormLabel>رمز عبور</FormLabel>
                    <FormControl>
                      <PasswordInput
                        value={field.value ?? ''}
                        onChange={field.onChange}
                        placeholder='رمز عبور'
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </>
          )}

          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='protocols'
              render={({ field }) => {
                const selected: string[] = field.value ?? []
                return (
                  <FormItem>
                    <FormLabel>پروتکل‌ها</FormLabel>
                    <FormDescription>
                      یک نام کاربری/رمز عبور مشترک روی RouterOS ساخته می‌شود
                      -- این حساب از نظر فنی می‌تواند با هر پروتکلی احراز
                      هویت شود (به‌جز IKEv2، که User Manager روتر مستقیماً از
                      آن پشتیبانی نمی‌کند و باید جداگانه روی روتر پیکربندی
                      شود). این انتخاب فقط تعیین می‌کند اطلاعات اتصال کدام
                      پروتکل‌ها در صفحه‌ی اشتراک این مشتری نمایش داده شود.
                    </FormDescription>
                    <div className='flex flex-wrap gap-4 pt-1'>
                      {USER_MANAGER_PROTOCOLS.map((protocol) => (
                        <label
                          key={protocol}
                          className='flex items-center gap-2 text-sm font-medium'
                        >
                          <Checkbox
                            checked={selected.includes(protocol)}
                            onCheckedChange={(checked) => {
                              if (checked) {
                                field.onChange([...selected, protocol])
                              } else {
                                field.onChange(
                                  selected.filter((p) => p !== protocol)
                                )
                              }
                            }}
                          />
                          {protocol.toUpperCase()}
                        </label>
                      ))}
                    </div>
                    <FormMessage />
                  </FormItem>
                )
              }}
            />
          </div>

          <FormField
            control={form.control}
            name='group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>گروه</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue
                        placeholder={
                          isGroupsLoading ? 'در حال بارگذاری...' : 'یک گروه انتخاب کنید'
                        }
                      />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {visibleGroups.map((group) => (
                      <SelectItem key={group.name} value={group.name}>
                        {group.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='profile'
            render={({ field }) => (
              <FormItem>
                <FormLabel>پروفایل</FormLabel>
                <Select onValueChange={field.onChange} value={field.value}>
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue
                        placeholder={
                          isProfilesLoading ? 'در حال بارگذاری...' : 'یک پروفایل انتخاب کنید'
                        }
                      />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {visibleProfiles.map((profile) => (
                      <SelectItem key={profile.name} value={profile.name}>
                        {profile.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='shared_users'
            render={({ field }) => (
              <FormItem>
                <FormLabel>کاربران هم‌زمان</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    step='1'
                    min='1'
                    value={field.value ?? 1}
                    onChange={(e) => field.onChange(Number(e.target.value))}
                  />
                </FormControl>
                <FormDescription>
                  حداکثر تعداد نشست‌های هم‌زمان برای این حساب.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='traffic_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>سقف ترافیک (گیگابایت)</FormLabel>
                <FormControl>
                  <Input
                    placeholder='برای نامحدود خالی بگذارید'
                    value={field.value ?? ''}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='expire_time'
            render={({ field }) => (
              <FormItem className='flex flex-col'>
                <FormLabel>زمان انقضا</FormLabel>
                <SimpleDatepicker
                  value={field.value ?? null}
                  onChange={field.onChange}
                />
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='md:col-span-2'>
            <FormField
              control={form.control}
              name='comment'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>توضیحات</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='توضیحات (اختیاری)'
                      value={field.value ?? ''}
                      onChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </div>
      </form>
    </Form>
  )
}
