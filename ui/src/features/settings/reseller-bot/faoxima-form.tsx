import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import {
  disableFaoxima,
  downloadFaoximaBackup,
  enableFaoxima,
  fetchFaoximaStatus,
  provisionFaoxima,
  removeFaoxima,
  restoreFaoximaBackup,
  updateFaoximaToken,
} from '@/api/faoxima.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

const statusLabels: Record<string, { label: string; variant: 'default' | 'secondary' | 'destructive' }> = {
  PROVISIONING: { label: 'در حال راه‌اندازی...', variant: 'secondary' },
  ENABLED: { label: 'فعال', variant: 'default' },
  DISABLED: { label: 'غیرفعال', variant: 'secondary' },
  ERROR: { label: 'خطا', variant: 'destructive' },
}

export function FaoximaForm() {
  const queryClient = useQueryClient()
  const { data: instance, isLoading } = useQuery({
    queryKey: ['faoxima_status'],
    queryFn: fetchFaoximaStatus,
  })

  const [botToken, setBotToken] = useState('')
  const [chatId, setChatId] = useState('')
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false)
  const restoreFileInputRef = useRef<HTMLInputElement>(null)

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ['faoxima_status'] })

  const provisionMutation = useMutation({
    mutationFn: provisionFaoxima,
    onSuccess: () => {
      toast.success('ربات ایکس شما با موفقیت راه‌اندازی شد.')
      invalidate()
      setBotToken('')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'راه‌اندازی ربات ناموفق بود.')),
  })

  const updateTokenMutation = useMutation({
    mutationFn: updateFaoximaToken,
    onSuccess: () => {
      toast.success('توکن ربات به‌روزرسانی شد.')
      invalidate()
      setBotToken('')
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'به‌روزرسانی توکن ناموفق بود.')),
  })

  const disableMutation = useMutation({
    mutationFn: disableFaoxima,
    onSuccess: () => {
      toast.success('ربات غیرفعال شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'غیرفعال‌سازی ناموفق بود.')),
  })

  const enableMutation = useMutation({
    mutationFn: enableFaoxima,
    onSuccess: () => {
      toast.success('ربات دوباره فعال شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'فعال‌سازی ناموفق بود.')),
  })

  const removeMutation = useMutation({
    mutationFn: removeFaoxima,
    onSuccess: () => {
      toast.success('ربات و تمام داده‌های آن حذف شد.')
      invalidate()
      setRemoveConfirmOpen(false)
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'حذف ربات ناموفق بود.')),
  })

  const backupMutation = useMutation({
    mutationFn: downloadFaoximaBackup,
    onSuccess: (blob) => {
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `faoxima-backup-${new Date().toISOString().slice(0, 10)}.sql`
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
      toast.success('فایل بکاپ دانلود شد.')
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'دریافت بکاپ ناموفق بود.')),
  })

  const restoreMutation = useMutation({
    mutationFn: restoreFaoximaBackup,
    onSuccess: () => {
      toast.success('بازیابی با موفقیت انجام شد.')
      invalidate()
    },
    onError: (err) => toast.error(backendErrorMessage(err, 'بازیابی ناموفق بود.')),
  })

  const handleRestoreFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    restoreMutation.mutate(file)
    e.target.value = ''
  }

  if (isLoading) {
    return <Skeleton className='h-64 w-full rounded-lg' />
  }

  // No instance yet -- show the initial provisioning form.
  if (!instance) {
    return (
      <div className='space-y-6'>
        <div className='space-y-2'>
          <Label htmlFor='faoxima-bot-token'>توکن ربات</Label>
          <Input
            id='faoxima-bot-token'
            type='password'
            value={botToken}
            onChange={(e) => setBotToken(e.target.value)}
            placeholder='توکن دریافت‌شده از @BotFather را وارد کنید'
          />
          <p className='text-muted-foreground text-xs'>
            یک ربات جدید از طریق @BotFather در تلگرام بسازید و توکن آن را
            اینجا وارد کنید. سورس ربات هرگز در اختیار شما قرار نمی‌گیرد.
          </p>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='faoxima-chat-id'>آیدی عددی چت مدیر (اختیاری)</Label>
          <Input
            id='faoxima-chat-id'
            value={chatId}
            onChange={(e) => setChatId(e.target.value)}
            placeholder='با @userinfobot پیدا کنید'
          />
        </div>

        <Button
          className='w-full'
          disabled={!botToken.trim() || provisionMutation.isPending}
          onClick={() =>
            provisionMutation.mutate({
              bot_token: botToken.trim(),
              admin_chat_id: chatId.trim() || undefined,
            })
          }
        >
          {provisionMutation.isPending
            ? 'در حال راه‌اندازی (ممکن است چند دقیقه طول بکشد)...'
            : 'راه‌اندازی ربات ایکس'}
        </Button>
      </div>
    )
  }

  const status = statusLabels[instance.status] ?? statusLabels.ERROR

  return (
    <div className='space-y-6'>
      <div className='flex items-center gap-2'>
        <Badge variant={status.variant}>{status.label}</Badge>
      </div>

      {instance.status === 'ERROR' && instance.error_message && (
        <div className='rounded-lg border border-destructive/50 p-3'>
          <p className='text-destructive text-xs'>{instance.error_message}</p>
        </div>
      )}

      <div className='space-y-2'>
        <Label htmlFor='faoxima-new-token'>تغییر توکن ربات</Label>
        <div className='flex gap-2'>
          <Input
            id='faoxima-new-token'
            type='password'
            value={botToken}
            onChange={(e) => setBotToken(e.target.value)}
            placeholder='توکن جدید (در صورت نیاز به تغییر)'
            className='flex-1'
          />
          <Button
            variant='outline'
            disabled={!botToken.trim() || updateTokenMutation.isPending}
            onClick={() => updateTokenMutation.mutate({ bot_token: botToken.trim() })}
          >
            {updateTokenMutation.isPending ? 'در حال ذخیره...' : 'ذخیره'}
          </Button>
        </div>
      </div>

      <div className='flex flex-wrap gap-2'>
        {instance.status === 'ENABLED' ? (
          <Button
            variant='outline'
            disabled={disableMutation.isPending}
            onClick={() => disableMutation.mutate()}
          >
            غیرفعال‌سازی ربات
          </Button>
        ) : (
          <Button
            variant='outline'
            disabled={enableMutation.isPending}
            onClick={() => enableMutation.mutate()}
          >
            فعال‌سازی ربات
          </Button>
        )}

        <Button
          variant='outline'
          disabled={backupMutation.isPending}
          onClick={() => backupMutation.mutate()}
        >
          {backupMutation.isPending ? 'در حال دریافت...' : 'دریافت بکاپ'}
        </Button>

        <input
          ref={restoreFileInputRef}
          type='file'
          accept='.sql'
          hidden
          onChange={handleRestoreFileChange}
        />
        <Button
          variant='outline'
          disabled={restoreMutation.isPending}
          onClick={() => restoreFileInputRef.current?.click()}
        >
          {restoreMutation.isPending ? 'در حال بازیابی...' : 'بازیابی از بکاپ'}
        </Button>

        <Button variant='destructive' onClick={() => setRemoveConfirmOpen(true)}>
          حذف کامل ربات
        </Button>
      </div>

      <AlertDialog open={removeConfirmOpen} onOpenChange={setRemoveConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>حذف کامل ربات ایکس</AlertDialogTitle>
            <AlertDialogDescription>
              این عمل ربات، پایگاه‌داده و تمام داده‌های فروش این ربات را
              برای همیشه حذف می‌کند و غیرقابل بازگشت است. اگر نیاز به
              نگهداری اطلاعات دارید، ابتدا از دکمه‌ی «دریافت بکاپ» استفاده
              کنید.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>انصراف</AlertDialogCancel>
            <AlertDialogAction onClick={() => removeMutation.mutate()}>
              حذف کامل
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
