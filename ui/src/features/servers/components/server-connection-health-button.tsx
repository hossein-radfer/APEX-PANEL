import { useState } from 'react'
import { IconPlugConnected } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { fetchServerConnectionHealth } from '@/api/servers.ts'
import { ServerConnectionHealth } from '@/schema/servers.ts'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const reasonLabelsFa: Record<string, string> = {
  ok: 'متصل',
  timeout: 'تایم‌اوت -- سرور میکروتیک در زمان مقرر پاسخ نداد',
  connection_refused: 'اتصال رد شد -- سرویس روی آن پورت در دسترس نیست',
  unauthorized: 'نام‌کاربری/رمز عبور سرور میکروتیک نادرست است',
  bad_status: 'پاسخ غیرمنتظره از سرور میکروتیک',
  not_configured: 'هیچ سروری در پنل تعریف نشده است',
  db_error: 'خطا در خواندن اطلاعات سرور از دیتابیس پنل',
  request_error: 'خطا در ساخت درخواست به سرور میکروتیک',
  connection_error: 'خطای اتصال نامشخص',
}

export function ServerConnectionHealthButton() {
  const [open, setOpen] = useState(false)
  const [isChecking, setIsChecking] = useState(false)
  const [result, setResult] = useState<ServerConnectionHealth | null>(null)

  const runCheck = async () => {
    setIsChecking(true)
    setOpen(true)
    try {
      const health = await fetchServerConnectionHealth()
      setResult(health)
    } catch {
      setResult({
        connected: false,
        reason: 'request_error',
        detail: 'ارسال درخواست بررسی اتصال به پنل ناموفق بود.',
      })
    } finally {
      setIsChecking(false)
    }
  }

  return (
    <>
      <Button
        variant='outline'
        className='space-x-1'
        disabled={isChecking}
        onClick={runCheck}
      >
        {isChecking ? (
          <Loader2Icon className='animate-spin' />
        ) : (
          <IconPlugConnected />
        )}
        <span>بررسی اتصال میکروتیک</span>
      </Button>

      <AlertDialog open={open} onOpenChange={setOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className='flex items-center gap-2'>
              نتیجه‌ی بررسی اتصال میکروتیک
              {result && (
                <Badge variant={result.connected ? 'default' : 'destructive'}>
                  {result.connected ? 'متصل' : 'قطع'}
                </Badge>
              )}
            </AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className='space-y-2 text-right'>
                {isChecking ? (
                  <span>در حال بررسی زنده‌ی اتصال...</span>
                ) : result ? (
                  <>
                    <p className='text-foreground font-medium'>
                      {reasonLabelsFa[result.reason] ?? result.reason}
                    </p>
                    <p className='text-muted-foreground text-xs'>
                      {result.detail}
                    </p>
                  </>
                ) : null}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button onClick={() => setOpen(false)}>بستن</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
