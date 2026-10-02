import { useEffect, useState } from 'react'
import { Peer } from '@/schema/peers.ts'
import {
  CalendarIcon,
  CheckIcon,
  ClipboardCopyIcon,
  LinkIcon,
  XIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { usePeerShareQuery } from '@/hooks/peers/usePeerShareQuery.ts'
import { useUpdatePeerShareExpireMutation } from '@/hooks/peers/useUpdatePeerShareExpireMutation.ts'
import { useUpdatePeerShareStatusMutation } from '@/hooks/peers/useUpdatePeerShareStatusMutation.ts'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { SimpleDatepicker } from '@/features/shared-components/simple-date-picker.tsx'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  currentRow: Peer
}

export function PeersShareDialog({ open, onOpenChange, currentRow }: Props) {
  const origin = location.origin

  const [shareLink, setShareLink] = useState('')

  const {
    data: shareData,
    isLoading,
    isFetching,
  } = usePeerShareQuery(currentRow.id, {
    enabled: open,
  })

  const updatePeerShareStatus = useUpdatePeerShareStatusMutation()
  const updatePeerShareExpire = useUpdatePeerShareExpireMutation()

  const isMutating =
    updatePeerShareStatus.isPending || updatePeerShareExpire.isPending

  // Deliberately does NOT auto-copy the link on load: clipboard writes
  // require a direct, synchronous user gesture in modern browsers (a
  // "transient activation"). A useEffect firing after an async query
  // resolves is not one -- the browser silently no-ops the write while
  // still resolving the promise/returning true from execCommand, so an
  // auto-copy-on-open here would show a false "copied" toast without
  // actually placing anything on the clipboard. The manual button below
  // (a real onClick handler) is the only reliable copy path.
  useEffect(() => {
    if (shareData?.uuid) {
      const link = `${origin}/share?shareId=${shareData.uuid}`
      setShareLink(link)
    }
  }, [origin, shareData])

  const handleCopy = () => {
    if (!shareLink) return
    copyToClipboard(shareLink).then((succeeded) => {
      if (succeeded) {
        toast.success('لینک اشتراک در کلیپ‌بورد کپی شد', { duration: 5000 })
      } else {
        toast.error('کپی لینک ناموفق بود', { duration: 5000 })
      }
    })
  }

  const handleExpireDateChange = async (value: string | null) => {
    await updatePeerShareExpire.mutateAsync({
      id: currentRow.id,
      expire_time: value,
    })

    toast.success(
      value
        ? 'تاریخ انقضا با موفقیت به‌روزرسانی شد'
        : 'تاریخ انقضا با موفقیت پاک شد',
      { duration: 5000 }
    )
  }

  const handleToggleStatus = async () => {
    await updatePeerShareStatus.mutateAsync(currentRow.id)

    toast.success(
      shareData?.is_shared
        ? 'اشتراک‌گذاری وایرگارد متوقف شد'
        : 'اشتراک‌گذاری وایرگارد شروع شد',
      { duration: 5000 }
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='space-y-2 sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>اشتراک‌گذاری وایرگارد</DialogTitle>
        </DialogHeader>

        <div className='gap-2 space-y-4'>
          {isLoading || isFetching ? (
            <>
              <Skeleton className='h-4 w-3/4' />
              <Skeleton className='h-10 w-full' />
              <Skeleton className='h-4 w-1/2' />
              <Skeleton className='h-10 w-full' />
              <Skeleton className='h-10 w-full' />
            </>
          ) : (
            <>
              {!shareData?.is_shared && (
                <Label>
                  در حال حاضر این وایرگارد به اشتراک گذاشته نشده است. با کلیک روی دکمه‌ی زیر می‌توانید اشتراک‌گذاری آن را شروع کنید.
                </Label>
              )}

              {shareData?.is_shared && (
                <div className='space-y-3'>
                  <Label className='flex items-center gap-1'>
                    <LinkIcon className='h-4 w-4 opacity-60' />
                    لینک اشتراک
                  </Label>
                  <div className='flex items-center gap-2'>
                    <Input value={shareLink ?? ''} readOnly />
                    <Button
                      variant='outline'
                      size='icon'
                      onClick={handleCopy}
                      disabled={isMutating}
                    >
                      <ClipboardCopyIcon className='h-4 w-4' />
                    </Button>
                  </div>
                </div>
              )}

              {shareData?.is_shared && (
                <div className='space-y-3'>
                  <Label className='flex items-center gap-1'>
                    <CalendarIcon className='h-4 w-4 opacity-60' />
                    تاریخ انقضا
                  </Label>
                  <SimpleDatepicker
                    value={shareData.expire_time ?? null}
                    onChange={handleExpireDateChange}
                    placeholder='تاریخ انقضا را انتخاب کنید'
                  />
                </div>
              )}

              {shareData?.is_shared ? (
                <Button
                  variant='destructive'
                  className='w-full'
                  onClick={handleToggleStatus}
                  disabled={isMutating}
                >
                  <XIcon className='mr-2 h-4 w-4' />
                  توقف اشتراک‌گذاری
                </Button>
              ) : (
                <Button
                  className='w-full text-white shadow-xs hover:bg-green-600/90 focus-visible:ring-green-600/20 dark:bg-green-600/60 dark:focus-visible:ring-green-600/40'
                  onClick={handleToggleStatus}
                  disabled={isMutating}
                >
                  <CheckIcon className='mr-2 h-4 w-4' />
                  شروع اشتراک‌گذاری
                </Button>
              )}
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
