import { useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import {
  IconDatabase,
  IconDownload,
  IconSend,
  IconUpload,
} from '@tabler/icons-react'
import { downloadBackup, sendInstantBackup, uploadRestoreBackup } from '@/api/backup.ts'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import ContentSection from '../components/content-section'
import { ApiKeyCard } from './api-key-card'
import { DatabaseSizeCard } from './database-size-card'
import { PortConfigCard } from './port-config-card'
import { SSLSettingsCard } from './ssl-settings-card'
import { SupportTokenCard } from './support-token-card'
import { SystemHealthCard } from './system-health-card'
import { TelegramBotCard } from './telegram-bot-card'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

export default function SettingsSystem() {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [stagedMessage, setStagedMessage] = useState<string | null>(null)

  const downloadMutation = useMutation({
    mutationFn: downloadBackup,
    onSuccess: () => toast.success('بکاپ دانلود شد.'),
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ایجاد بکاپ ناموفق بود.')),
  })

  const instantBackupMutation = useMutation({
    mutationFn: sendInstantBackup,
    onSuccess: () =>
      toast.success('بکاپ به تلگرام ارسال شد.'),
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ارسال بکاپ به تلگرام ناموفق بود.')),
  })

  const restoreMutation = useMutation({
    mutationFn: uploadRestoreBackup,
    onSuccess: (result) => {
      toast.success('فایل بکاپ ثبت شد. برای اعمال آن، پنل را ری‌استارت کنید.')
      setStagedMessage(result.message)
      if (fileInputRef.current) fileInputRef.current.value = ''
    },
    onError: (err) => {
      toast.error(backendErrorMessage(err, 'ثبت بکاپ برای بازیابی ناموفق بود.'))
      if (fileInputRef.current) fileInputRef.current.value = ''
    },
  })

  const handleFileSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    restoreMutation.mutate(file)
  }

  return (
    <ContentSection
      title='سیستم'
      desc='تهیه و بازیابی بکاپ پایگاه داده پنل. مخصوص مدیر.'
    >
      <div className='space-y-6'>
        <SystemHealthCard />

        <PortConfigCard />

        <SSLSettingsCard />

        <SupportTokenCard />

        <ApiKeyCard />

        <TelegramBotCard />

        <DatabaseSizeCard />

        <Card>
          <CardHeader className='flex flex-row items-center gap-3 pb-2'>
            <IconDatabase className='h-5 w-5' />
            <div>
              <CardTitle className='text-lg'>دانلود بکاپ</CardTitle>
              <CardDescription>
                یک نسخه‌ی جدید و یکپارچه از پایگاه داده‌ی فعال می‌سازد و آن را
                دانلود می‌کند.
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <Button
              onClick={() => downloadMutation.mutate()}
              disabled={downloadMutation.isPending}
              className='gap-2'
            >
              <IconDownload className='h-4 w-4' />
              {downloadMutation.isPending ? 'در حال آماده‌سازی...' : 'دانلود بکاپ'}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-row items-center gap-3 pb-2'>
            <IconSend className='h-5 w-5' />
            <div>
              <CardTitle className='text-lg'>بکاپ فوری</CardTitle>
              <CardDescription>
                یک بکاپ جدید می‌سازد و همین حالا، بدون نیاز به منتظر ماندن
                برای بکاپ روزانه‌ی زمان‌بندی‌شده، آن را به چت(های) تلگرام
                مدیر ارسال می‌کند.
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent>
            <Button
              onClick={() => instantBackupMutation.mutate()}
              disabled={instantBackupMutation.isPending}
              className='gap-2'
            >
              <IconSend className='h-4 w-4' />
              {instantBackupMutation.isPending
                ? 'در حال ارسال...'
                : 'ارسال بکاپ فوری به تلگرام'}
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-row items-center gap-3 pb-2'>
            <IconUpload className='h-5 w-5' />
            <div>
              <CardTitle className='text-lg'>بازیابی بکاپ</CardTitle>
              <CardDescription>
                یک فایل بکاپ دانلودشده‌ی قبلی را برای انتقال پنل به سرور
                جدید آپلود کنید.
              </CardDescription>
            </div>
          </CardHeader>
          <CardContent className='space-y-4'>
            <Alert>
              <AlertTitle>برای اعمال، ری‌استارت لازم است</AlertTitle>
              <AlertDescription>
                آپلود فایل فقط آن را روی سرور ثبت می‌کند — پایگاه داده‌ی
                فعال بدون تغییر به کار خود ادامه می‌دهد تا زمانی که فرآیند
                پنل ری‌استارت شود. در ری‌استارت بعدی (پیش از باز شدن پایگاه
                داده)، فایل ثبت‌شده به‌طور خودکار جایگزین فایل فعال می‌شود.
                مرحله‌ی جداگانه‌ای وجود ندارد: ری‌استارت کردن، خودِ بازیابی
                است.
              </AlertDescription>
            </Alert>

            <input
              ref={fileInputRef}
              type='file'
              accept='.db'
              onChange={handleFileSelected}
              disabled={restoreMutation.isPending}
              className='text-sm'
            />

            {restoreMutation.isPending && (
              <p className='text-muted-foreground text-sm'>در حال آپلود و بررسی...</p>
            )}

            {stagedMessage && (
              <Alert>
                <AlertTitle>بکاپ ثبت شد</AlertTitle>
                <AlertDescription>{stagedMessage}</AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      </div>
    </ContentSection>
  )
}
