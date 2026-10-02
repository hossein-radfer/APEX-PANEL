import { useEffect, useState } from 'react'
import { V2RayPackage } from '@/schema/v2ray.ts'
import {
  CalendarIcon,
  CheckIcon,
  ClipboardCopyIcon,
  LinkIcon,
  RssIcon,
  XIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { useV2RayPackageShareQuery } from '@/hooks/v2ray/useV2RayPackageShareQuery.ts'
import { useUpdateV2RayPackageShareExpireMutation } from '@/hooks/v2ray/useUpdateV2RayPackageShareExpireMutation.ts'
import { useUpdateV2RayPackageShareStatusMutation } from '@/hooks/v2ray/useUpdateV2RayPackageShareStatusMutation.ts'
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
  currentRow: V2RayPackage
}

export function V2RayShareDialog({ open, onOpenChange, currentRow }: Props) {
  const origin = location.origin

  const [shareLink, setShareLink] = useState('')
  const [subscriptionLink, setSubscriptionLink] = useState('')

  const {
    data: shareData,
    isLoading,
    isFetching,
  } = useV2RayPackageShareQuery(currentRow.id, { enabled: open })

  const updateShareStatus = useUpdateV2RayPackageShareStatusMutation()
  const updateShareExpire = useUpdateV2RayPackageShareExpireMutation()

  const isMutating = updateShareStatus.isPending || updateShareExpire.isPending

  // Deliberately does NOT auto-copy the link on load -- see the identical
  // comment in account-share-dialog.tsx: clipboard writes require a direct
  // user gesture, and a useEffect firing after an async query resolves
  // isn't one. The manual button below is the only reliable copy path.
  useEffect(() => {
    if (shareData?.uuid) {
      setShareLink(`${origin}/v2ray-share?shareId=${shareData.uuid}`)
      setSubscriptionLink(`${origin}/api/v2ray-sub/${shareData.uuid}`)
    }
  }, [origin, shareData])

  const handleCopy = () => {
    if (!shareLink) {
      toast.error('لینک اشتراک‌گذاری هنوز آماده نیست -- کمی بعد دوباره تلاش کنید', {
        duration: 5000,
      })
      return
    }
    copyToClipboard(shareLink).then((succeeded) => {
      if (succeeded) {
        toast.success('لینک اشتراک‌گذاری در کلیپ‌بورد کپی شد', { duration: 5000 })
      } else {
        toast.error('کپی کردن لینک ناموفق بود', { duration: 5000 })
      }
    })
  }

  const handleCopySubscriptionLink = () => {
    if (!subscriptionLink) {
      toast.error('لینک سابسکریپشن هنوز آماده نیست -- کمی بعد دوباره تلاش کنید', {
        duration: 5000,
      })
      return
    }
    copyToClipboard(subscriptionLink).then((succeeded) => {
      if (succeeded) {
        toast.success('لینک سابسکریپشن در کلیپ‌بورد کپی شد', {
          duration: 5000,
        })
      } else {
        toast.error('کپی کردن لینک ناموفق بود', { duration: 5000 })
      }
    })
  }

  const handleExpireDateChange = async (value: string | null) => {
    await updateShareExpire.mutateAsync({
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
    await updateShareStatus.mutateAsync(currentRow.id)

    toast.success(
      shareData?.is_shared
        ? 'اشتراک‌گذاری بسته با موفقیت متوقف شد'
        : 'اشتراک‌گذاری بسته با موفقیت آغاز شد',
      { duration: 5000 }
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='space-y-2 sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>اشتراک‌گذاری بسته</DialogTitle>
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
                  در حال حاضر، این بسته به اشتراک گذاشته نشده است. می‌توانید
                  با کلیک روی دکمه‌ی زیر اشتراک‌گذاری آن را شروع کنید.
                </Label>
              )}

              {shareData?.is_shared && (
                <div className='space-y-3'>
                  <Label className='flex items-center gap-1'>
                    <LinkIcon className='h-4 w-4 opacity-60' />
                    لینک اشتراک‌گذاری
                  </Label>
                  <div className='flex items-center gap-2'>
                    <Input value={shareLink ?? ''} readOnly />
                    <Button
                      variant='outline'
                      size='icon'
                      onClick={handleCopy}
                      disabled={isMutating || !shareLink}
                    >
                      <ClipboardCopyIcon className='h-4 w-4' />
                    </Button>
                  </div>
                </div>
              )}

              {shareData?.is_shared && (
                <div className='space-y-3'>
                  <Label className='flex items-center gap-1'>
                    <RssIcon className='h-4 w-4 opacity-60' />
                    لینک سابسکریپشن
                  </Label>
                  <div className='flex items-center gap-2'>
                    <Input value={subscriptionLink ?? ''} readOnly />
                    <Button
                      variant='outline'
                      size='icon'
                      onClick={handleCopySubscriptionLink}
                      disabled={isMutating || !subscriptionLink}
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
                    placeholder='یک تاریخ انقضا انتخاب کنید'
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
