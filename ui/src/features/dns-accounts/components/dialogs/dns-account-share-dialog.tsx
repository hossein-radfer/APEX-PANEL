import { useEffect, useState } from 'react'
import { DNSAccount } from '@/schema/dns-account.ts'
import {
  CalendarIcon,
  CheckIcon,
  ClipboardCopyIcon,
  LinkIcon,
  XIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { useDNSAccountShareQuery } from '@/hooks/dns-account/useDNSAccountShareQuery.ts'
import { useUpdateDNSAccountShareExpireMutation } from '@/hooks/dns-account/useUpdateDNSAccountShareExpireMutation.ts'
import { useUpdateDNSAccountShareStatusMutation } from '@/hooks/dns-account/useUpdateDNSAccountShareStatusMutation.ts'
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
  currentRow: DNSAccount
}

// Mirrors V2RayShareDialog exactly, minus the subscription-link section --
// DNS has no VPN client subscription format, only the "Register my IP"
// status page reached by the same share link.
export function DNSAccountShareDialog({ open, onOpenChange, currentRow }: Props) {
  const origin = location.origin

  const [shareLink, setShareLink] = useState('')

  const {
    data: shareData,
    isLoading,
    isFetching,
  } = useDNSAccountShareQuery(currentRow.id, { enabled: open })

  const updateShareStatus = useUpdateDNSAccountShareStatusMutation()
  const updateShareExpire = useUpdateDNSAccountShareExpireMutation()

  const isMutating = updateShareStatus.isPending || updateShareExpire.isPending

  // Deliberately does NOT auto-copy the link on load -- clipboard writes
  // require a direct user gesture, and a useEffect firing after an async
  // query resolves isn't one. Same convention as V2RayShareDialog.
  useEffect(() => {
    if (shareData?.uuid) {
      setShareLink(`${origin}/dns-share?shareId=${shareData.uuid}`)
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
        ? 'اشتراک‌گذاری حساب با موفقیت متوقف شد'
        : 'اشتراک‌گذاری حساب با موفقیت آغاز شد',
      { duration: 5000 }
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='space-y-2 sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>اشتراک‌گذاری حساب</DialogTitle>
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
                  در حال حاضر، این حساب به اشتراک گذاشته نشده است. می‌توانید
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
                  <p className='text-muted-foreground text-xs'>
                    این لینک برای مشتری، وضعیت مصرف و دکمه‌ی «ثبت IP من» را
                    نمایش می‌دهد.
                  </p>
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
