import { useEffect, useState } from 'react'
import { USER_MANAGER_PROTOCOLS, UserManagerProtocolEnum } from '@/schema/user-manager.ts'
import { UserPlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import { z } from 'zod'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { useBulkCreateUserManagerAccountsMutation } from '@/hooks/user-manager/useBulkCreateUserManagerAccountsMutation.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
import { useServersListQuery } from '@/hooks/servers/useServersListQuery.ts'
import { useServerEndpointsQuery } from '@/hooks/servers/useServerEndpointsQuery.ts'
import { Button } from '@/components/ui/button.tsx'
import { Checkbox } from '@/components/ui/checkbox.tsx'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog.tsx'
import { Input } from '@/components/ui/input.tsx'
import { Label } from '@/components/ui/label.tsx'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select.tsx'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
}

// BulkCreateDialog mirrors PeersBulkCreateDialog/V2RayBulkCreateDialog's
// exact create-then-download flow, adapted for User Manager accounts:
// create N accounts at once (random username+password, shared group/
// profile/protocols/traffic-limit/duration), then hand the batch straight
// to a downloadable .txt file with each account's credentials and share
// link.
export function BulkCreateDialog({ open, onOpenChange }: Props) {
  const [count, setCount] = useState<number | string>(10)
  const [group, setGroup] = useState('')
  const [profile, setProfile] = useState('')
  const [protocols, setProtocols] = useState<z.infer<typeof UserManagerProtocolEnum>[]>(['l2tp'])
  const [durationDays, setDurationDays] = useState<number | string>('')
  const [trafficLimit, setTrafficLimit] = useState<number | string>('')
  const [selectedServerId, setSelectedServerId] = useState<number | undefined>(
    undefined
  )

  // A reseller cannot call the admin server list (it carries management
  // actions they aren't allowed to use and credentials they must not see),
  // so they fall back to the credential-free endpoints-only query -- same
  // pattern as AccountForm/use-peer-defaults.tsx. Without a server_id here,
  // the backend's resolveServerID has no way to pick a server once more
  // than one is registered, and this dialog's bulk-create call fails
  // outright for any reseller.
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  const { data: adminServersList = [] } = useServersListQuery(open && !isReseller)
  const { data: resellerServerEndpoints = [] } = useServerEndpointsQuery(
    open && isReseller
  )
  const serversList = isReseller ? resellerServerEndpoints : adminServersList

  const { data: groups = [] } = useUserManagerGroupsQuery(selectedServerId)
  const { data: profiles = [] } = useUserManagerProfilesQuery(selectedServerId)

  useEffect(() => {
    if (open && selectedServerId === undefined && serversList.length === 1) {
      setSelectedServerId(serversList[0].id)
    }
  }, [open, selectedServerId, serversList])

  useEffect(() => {
    if (open && !group && groups.length > 0) setGroup(groups[0].name)
    if (open && !profile && profiles.length > 0) setProfile(profiles[0].name)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, groups, profiles])

  const bulkCreate = useBulkCreateUserManagerAccountsMutation()

  const resetForm = () => {
    setCount(10)
    setGroup('')
    setProfile('')
    setProtocols(['l2tp'])
    setDurationDays('')
    setTrafficLimit('')
    setSelectedServerId(undefined)
  }

  const handleClose = (isOpen: boolean) => {
    onOpenChange(isOpen)
    if (!isOpen) setTimeout(resetForm, 500)
  }

  const toggleProtocol = (protocol: z.infer<typeof UserManagerProtocolEnum>, checked: boolean) => {
    setProtocols((prev) =>
      checked ? [...prev, protocol] : prev.filter((p) => p !== protocol)
    )
  }

  const downloadTxt = (
    accounts: { username: string; password: string; uuid: string }[]
  ) => {
    const origin = window.location.origin
    const total = accounts.length
    const divider = '='.repeat(40)
    const header = [
      divider,
      'لیست حساب‌های ساخته‌شده - User Manager',
      `تعداد: ${total}`,
      `تاریخ: ${new Date().toLocaleDateString('fa-IR')}`,
      divider,
      '',
    ].join('\n')
    const lines = accounts.map((account, i) => {
      const link = `${origin}/user-manager-share?shareId=${account.uuid}`
      return [
        `حساب ${i + 1} از ${total}`,
        '-'.repeat(24),
        `نام کاربری : ${account.username}`,
        `رمز عبور   : ${account.password}`,
        `لینک اشتراک: ${link}`,
      ].join('\n')
    })
    const content = header + lines.join('\n\n' + divider + '\n\n') + '\n' + divider
    const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `حساب‌های-یوزرمنیجر-${new Date().toISOString().split('T')[0]}.txt`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  const handleSubmit = async () => {
    const parsedCount = Number(count)
    const parsedDuration = Number(durationDays)

    if (!Number.isFinite(parsedCount) || parsedCount < 1 || parsedCount > 500) {
      toast.error('تعداد باید بین ۱ تا ۵۰۰ باشد.')
      return
    }
    if (!group) {
      toast.error('یک گروه انتخاب کنید.')
      return
    }
    if (!profile) {
      toast.error('یک پروفایل انتخاب کنید.')
      return
    }
    if (protocols.length === 0) {
      toast.error('حداقل یک پروتکل انتخاب کنید.')
      return
    }
    if (!Number.isFinite(parsedDuration) || parsedDuration <= 0) {
      toast.error('مدت زمان باید یک عدد مثبت (روز) باشد.')
      return
    }
    if (serversList.length > 1 && selectedServerId === undefined) {
      toast.error('یک سرور انتخاب کنید.')
      return
    }

    try {
      const created = await bulkCreate.mutateAsync({
        count: parsedCount,
        group,
        profile,
        protocols,
        duration_days: parsedDuration,
        traffic_limit: trafficLimit ? String(trafficLimit) : undefined,
        server_id: selectedServerId,
      })

      downloadTxt(
        created.map((a) => ({
          username: a.username,
          password: a.password,
          uuid: a.uuid,
        }))
      )

      toast.success(`${created.length} اکانت با موفقیت ساخته شد.`)
      handleClose(false)
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'ساخت گروهی اکانت‌ها ناموفق بود.'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>ساخت گروهی حساب</DialogTitle>
          <DialogDescription>
            چندین حساب User Manager را یکجا بسازید، سپس دسته‌ی ساخته‌شده را
            به‌صورت فایل متنی همراه با اطلاعات ورود و لینک اشتراک هر حساب
            دانلود کنید.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4'>
          {serversList.length > 1 && (
            <div className='space-y-2'>
              <Label>سرور</Label>
              <Select
                value={selectedServerId ? String(selectedServerId) : undefined}
                onValueChange={(value) => setSelectedServerId(Number(value))}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder='یک سرور انتخاب کنید' />
                </SelectTrigger>
                <SelectContent>
                  {serversList.map((server) => (
                    <SelectItem key={server.id} value={String(server.id)}>
                      {server.name} ({server.ip_address})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>تعداد</Label>
              <Input
                type='number'
                step='1'
                min='1'
                max='500'
                value={count}
                onChange={(e) => setCount(e.target.value)}
              />
            </div>
            <div className='space-y-2'>
              <Label>مدت زمان (روز)</Label>
              <Input
                type='number'
                step='1'
                min='1'
                placeholder='مثلاً 30'
                value={durationDays}
                onChange={(e) => setDurationDays(e.target.value)}
              />
            </div>
          </div>

          <div className='grid grid-cols-1 gap-x-3 gap-y-4 md:grid-cols-2'>
            <div className='space-y-2'>
              <Label>گروه</Label>
              <Select value={group || undefined} onValueChange={setGroup}>
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder='یک گروه انتخاب کنید' />
                </SelectTrigger>
                <SelectContent>
                  {groups.map((g) => (
                    <SelectItem key={g.name} value={g.name}>
                      {g.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-2'>
              <Label>پروفایل</Label>
              <Select value={profile || undefined} onValueChange={setProfile}>
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder='یک پروفایل انتخاب کنید' />
                </SelectTrigger>
                <SelectContent>
                  {profiles.map((p) => (
                    <SelectItem key={p.name} value={p.name}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className='space-y-2 rounded-lg border p-4'>
            <p className='text-sm font-medium'>پروتکل‌ها</p>
            <div className='grid gap-2 md:grid-cols-3'>
              {USER_MANAGER_PROTOCOLS.map((protocol) => (
                <label key={protocol} className='flex items-center gap-2 text-sm'>
                  <Checkbox
                    checked={protocols.includes(protocol)}
                    onCheckedChange={(checked) =>
                      toggleProtocol(protocol, Boolean(checked))
                    }
                  />
                  {protocol}
                </label>
              ))}
            </div>
          </div>

          <div className='space-y-2'>
            <Label>سقف ترافیک (گیگابایت، اختیاری)</Label>
            <Input
              type='number'
              step='0.01'
              min='0'
              value={trafficLimit}
              onChange={(e) => setTrafficLimit(e.target.value)}
              placeholder='برای نامحدود خالی بگذارید'
            />
          </div>
        </div>

        <DialogFooter>
          <Button
            onClick={handleSubmit}
            disabled={bulkCreate.isPending}
            className='gap-2'
          >
            {bulkCreate.isPending ? (
              'در حال انجام...'
            ) : (
              <>
                <UserPlusIcon className='h-4 w-4' />
                ساخت و دانلود
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
