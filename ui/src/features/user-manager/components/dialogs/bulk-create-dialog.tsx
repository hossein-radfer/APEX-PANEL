import { useEffect, useState } from 'react'
import { USER_MANAGER_PROTOCOLS, UserManagerProtocolEnum } from '@/schema/user-manager.ts'
import { UserPlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import { z } from 'zod'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useBulkCreateUserManagerAccountsMutation } from '@/hooks/user-manager/useBulkCreateUserManagerAccountsMutation.ts'
import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
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

  const { data: groups = [] } = useUserManagerGroupsQuery()
  const { data: profiles = [] } = useUserManagerProfilesQuery()

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
    const lines = accounts.map((account, i) => {
      const link = `${origin}/user-manager-share?shareId=${account.uuid}`
      return `اکانت ${i + 1} از ${total}\nیوزرنیم 👤: ${account.username}\nپسورد 🔒: ${account.password}\nساب 🔗: ${link}`
    })
    const blob = new Blob([lines.join('\n\n')], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `user-manager-accounts-${new Date().toISOString().split('T')[0]}.txt`
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

    try {
      const created = await bulkCreate.mutateAsync({
        count: parsedCount,
        group,
        profile,
        protocols,
        duration_days: parsedDuration,
        traffic_limit: trafficLimit ? String(trafficLimit) : undefined,
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
