import { useRef, useState } from 'react'
import { UserManagerAccount } from '@/schema/user-manager.ts'
import { DownloadIcon, UploadIcon } from 'lucide-react'
import { toast } from 'sonner'
import { useUploadUserManagerAccountConfigMutation } from '@/hooks/user-manager/useUploadUserManagerAccountConfigMutation.ts'
import { useUploadUserManagerAccountConfigForResellerMutation } from '@/hooks/user-manager/useUploadUserManagerAccountConfigForResellerMutation.ts'
import { fetchUserManagerAccountConfig } from '@/api/user-manager.ts'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  currentRow: UserManagerAccount
  // When set, an admin is managing this account's config file on behalf of
  // a specific reseller (admin "Reseller User Manager" page).
  targetResellerId?: number
}

export function AccountConfigDialog({
  open,
  onOpenChange,
  currentRow,
  targetResellerId,
}: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [isDownloading, setIsDownloading] = useState(false)

  const { mutateAsync: uploadConfig, isPending: isUploadPending } =
    useUploadUserManagerAccountConfigMutation()
  const {
    mutateAsync: uploadConfigForReseller,
    isPending: isUploadForResellerPending,
  } = useUploadUserManagerAccountConfigForResellerMutation()

  const isUploading =
    targetResellerId !== undefined
      ? isUploadForResellerPending
      : isUploadPending

  const handleFileSelected = async (
    e: React.ChangeEvent<HTMLInputElement>
  ) => {
    const file = e.target.files?.[0]
    if (!file) return

    try {
      if (targetResellerId !== undefined) {
        await uploadConfigForReseller({
          resellerId: targetResellerId,
          accountId: currentRow.id,
          file,
        })
      } else {
        await uploadConfig({ accountId: currentRow.id, file })
      }
      toast.success('فایل کانفیگ با موفقیت آپلود شد.', { duration: 5000 })
    } catch {
      toast.error('آپلود فایل کانفیگ ناموفق بود. دوباره تلاش کنید.')
    } finally {
      e.target.value = ''
    }
  }

  const handleDownload = async () => {
    setIsDownloading(true)
    try {
      const blob = await fetchUserManagerAccountConfig(currentRow.id)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${currentRow.username || 'account'}.ovpn`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      URL.revokeObjectURL(url)
    } catch {
      toast.error('دانلود فایل کانفیگ ناموفق بود.')
    } finally {
      setIsDownloading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='space-y-4 sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>فایل کانفیگ</DialogTitle>
          <DialogDescription>
            یک فایل کانفیگ (مثلاً یک پروفایل .ovpn برای OpenVPN) برای این
            حساب آپلود کنید. مشتری می‌تواند آن را از صفحه‌ی اشتراک خود
            دانلود کند.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-3'>
          <Label>
            {currentRow.has_config_file
              ? 'جایگزینی فایل کانفیگ'
              : 'آپلود فایل کانفیگ'}
          </Label>
          <input
            ref={fileInputRef}
            type='file'
            className='hidden'
            onChange={handleFileSelected}
          />
          <Button
            variant='outline'
            className='w-full'
            disabled={isUploading}
            onClick={() => fileInputRef.current?.click()}
          >
            <UploadIcon className='mr-2 h-4 w-4' />
            {isUploading ? 'در حال آپلود...' : 'انتخاب فایل'}
          </Button>

          {currentRow.has_config_file && (
            <Button
              variant='secondary'
              className='w-full'
              disabled={isDownloading}
              onClick={handleDownload}
            >
              <DownloadIcon className='mr-2 h-4 w-4' />
              {isDownloading ? 'در حال دانلود...' : 'دانلود کانفیگ فعلی'}
            </Button>
          )}

          {!currentRow.has_config_file && (
            <p className='text-muted-foreground text-sm'>
              هنوز فایل کانفیگی برای این حساب آپلود نشده است.
            </p>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
