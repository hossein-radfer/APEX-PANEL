import { useEffect, useState } from 'react'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import {
  CalendarIcon,
  CheckIcon,
  ClipboardCopyIcon,
  LinkIcon,
  XIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { useUserManagerAccountShareQuery } from '@/hooks/user-manager/useUserManagerAccountShareQuery.ts'
import { useUpdateUserManagerAccountShareExpireMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountShareExpireMutation.ts'
import { useUpdateUserManagerAccountShareStatusMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountShareStatusMutation.ts'
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
  currentRow: UserManagerAccount
}

export function AccountShareDialog({ open, onOpenChange, currentRow }: Props) {
  const origin = location.origin

  const [shareLink, setShareLink] = useState('')

  const {
    data: shareData,
    isLoading,
    isFetching,
  } = useUserManagerAccountShareQuery(currentRow.id, { enabled: open })

  const updateShareStatus = useUpdateUserManagerAccountShareStatusMutation()
  const updateShareExpire = useUpdateUserManagerAccountShareExpireMutation()

  const isMutating = updateShareStatus.isPending || updateShareExpire.isPending

  // Deliberately does NOT auto-copy the link on load -- see the identical
  // comment in peers-share-dialog.tsx: clipboard writes require a direct
  // user gesture, and a useEffect firing after an async query resolves
  // isn't one. The manual button below is the only reliable copy path.
  useEffect(() => {
    if (shareData?.uuid) {
      const link = `${origin}/user-manager-share?shareId=${shareData.uuid}`
      setShareLink(link)
    }
  }, [origin, shareData])

  const handleCopy = () => {
    if (!shareLink) {
      // Silently no-op-ing here (the previous behavior) looks identical
      // to a broken button from the user's side -- always give feedback,
      // even for "there's nothing to copy yet" (e.g. the share status
      // query hasn't resolved, or resolved with no uuid).
      toast.error('لینک اشتراک هنوز آماده نیست -- لحظاتی دیگر دوباره تلاش کنید', { duration: 5000 })
      return
    }
    copyToClipboard(shareLink).then((succeeded) => {
      if (succeeded) {
        toast.success('لینک اشتراک در کلیپ‌بورد کپی شد', { duration: 5000 })
      } else {
        toast.error('کپی لینک ناموفق بود', { duration: 5000 })
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
        ? 'اشتراک‌گذاری حساب متوقف شد'
        : 'اشتراک‌گذاری حساب شروع شد',
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
                  در حال حاضر این حساب به اشتراک گذاشته نشده است. با کلیک روی دکمه‌ی زیر می‌توانید اشتراک‌گذاری آن را شروع کنید.
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
