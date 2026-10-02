import { useEffect, useMemo, useState } from 'react'
import { UserPlusIcon } from 'lucide-react'
import { toast } from 'sonner'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { useBulkCreatePeersMutation } from '@/hooks/peers/useBulkCreatePeersMutation.ts'
import { useAssignedInterfacesQuery } from '@/hooks/resellers/useAssignedInterfacesQuery.ts'
import { useInterfacesListQuery } from '@/hooks/interfaces/useInterfacesListQuery.ts'
import { useServerEndpointsQuery } from '@/hooks/servers/useServerEndpointsQuery.ts'
import { useServersListQuery } from '@/hooks/servers/useServersListQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Button } from '@/components/ui/button.tsx'
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

// PeersBulkCreateDialog mirrors V2RayBulkCreateDialog's exact create-then-
// export flow, adapted for WireGuard peers: create N peers at once (random
// name, shared interface/endpoint/traffic-limit/bandwidth), then hand the
// batch straight to a downloadable .txt file -- built client-side here
// since every field the txt needs (name, share link) is already present in
// BulkCreatePeers' own response, unlike V2Ray's export which needed a
// separate server round-trip for xlsx generation.
export function PeersBulkCreateDialog({ open, onOpenChange }: Props) {
  const [count, setCount] = useState<number | string>(10)
  const [interfaceId, setInterfaceId] = useState<number | ''>('')
  const [endpoint, setEndpoint] = useState('')
  const [durationDays, setDurationDays] = useState<number | string>('')
  const [trafficLimit, setTrafficLimit] = useState<number | string>('')

  const role = useAuthStore((state) => state.auth.admin?.role)
  const isReseller = role === 'reseller'
  const resellerId = useAuthStore((state) => state.auth.admin?.reseller_id)

  const { data: allInterfaces = [] } = useInterfacesListQuery(!isReseller)
  const { data: assignedInterfaceIds } = useAssignedInterfacesQuery(
    isReseller ? (resellerId ?? undefined) : undefined
  )
  const interfacesList = useMemo(() => {
    if (!isReseller) return allInterfaces
    const allowedIds = new Set(assignedInterfaceIds ?? [])
    return allInterfaces.filter((iface) => allowedIds.has(iface.id))
  }, [isReseller, allInterfaces, assignedInterfaceIds])

  const { data: adminServers = [] } = useServersListQuery(!isReseller)
  const { data: resellerServerEndpoints = [] } = useServerEndpointsQuery(isReseller)
  const serversList = isReseller ? resellerServerEndpoints : adminServers

  useEffect(() => {
    if (open && interfaceId === '' && interfacesList.length > 0) {
      setInterfaceId(interfacesList[0].id)
    }
    if (open && endpoint === '' && serversList.length > 0) {
      setEndpoint(serversList[0].ip_address)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, interfacesList, serversList])

  const bulkCreate = useBulkCreatePeersMutation()

  const resetForm = () => {
    setCount(10)
    setInterfaceId('')
    setEndpoint('')
    setDurationDays('')
    setTrafficLimit('')
  }

  const handleClose = (isOpen: boolean) => {
    onOpenChange(isOpen)
    if (!isOpen) setTimeout(resetForm, 500)
  }

  const downloadTxt = (
    peers: { name: string; uuid: string }[]
  ) => {
    const origin = window.location.origin
    const total = peers.length
    const lines = peers.map((peer, i) => {
      const link = `${origin}/share?shareId=${peer.uuid}`
      return `اکانت ${i + 1} از ${total}\nنام: ${peer.name}\nلینک ساب 🔗: ${link}`
    })
    const blob = new Blob([lines.join('\n\n')], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `wireguard-peers-${new Date().toISOString().split('T')[0]}.txt`
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
    if (!interfaceId) {
      toast.error('یک اینترفیس انتخاب کنید.')
      return
    }
    if (!endpoint) {
      toast.error('آدرس Endpoint را وارد کنید.')
      return
    }
    if (!Number.isFinite(parsedDuration) || parsedDuration <= 0) {
      toast.error('مدت زمان باید یک عدد مثبت (روز) باشد.')
      return
    }

    try {
      const created = await bulkCreate.mutateAsync({
        count: parsedCount,
        interface_id: interfaceId,
        endpoint,
        duration_days: parsedDuration,
        traffic_limit: trafficLimit ? String(trafficLimit) : undefined,
      })

      downloadTxt(created.map((p) => ({ name: p.name, uuid: p.uuid })))

      toast.success(`${created.length} وایرگارد با موفقیت ساخته شد.`)
      handleClose(false)
    } catch (error) {
      toast.error(getApiErrorMessage(error, 'ساخت گروهی وایرگارد ناموفق بود.'))
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>ساخت گروهی وایرگارد</DialogTitle>
          <DialogDescription>
            چندین وایرگارد را یکجا بسازید، سپس دسته‌ی ساخته‌شده را به‌صورت فایل متنی همراه با لینک اشتراک هر وایرگارد دانلود کنید.
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

          <div className='space-y-2'>
            <Label>اینترفیس</Label>
            <Select
              value={interfaceId === '' ? undefined : String(interfaceId)}
              onValueChange={(value) => setInterfaceId(Number(value))}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='یک اینترفیس انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {interfacesList.map((iface) => (
                  <SelectItem key={iface.id} value={String(iface.id)}>
                    {iface.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>آدرس اتصال (Endpoint)</Label>
            <Input
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              placeholder='آدرس IP سرور'
            />
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
