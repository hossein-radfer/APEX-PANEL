import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconLock } from '@tabler/icons-react'
import { fetchSSLSettings, updateSSLSettings } from '@/api/ssl-settings.ts'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

// SSL is a "bring your own certificate" tab, not an ACME client -- the
// admin runs Certbot (or any other tool) on the server themselves, outside
// this panel entirely, and just pastes the resulting certificate/key file
// PATHS here. The panel only ever reads those two files at its own next
// restart; it never issues or renews anything.
export function SSLSettingsCard() {
  const queryClient = useQueryClient()
  const { data: settings, isLoading } = useQuery({
    queryKey: ['ssl_settings'],
    queryFn: fetchSSLSettings,
  })

  const [domain, setDomain] = useState('')
  const [certificatePath, setCertificatePath] = useState('')
  const [privateKeyPath, setPrivateKeyPath] = useState('')
  const [enabled, setEnabled] = useState(false)

  useEffect(() => {
    if (!settings) return
    setDomain(settings.domain)
    setCertificatePath(settings.certificate_path)
    setPrivateKeyPath(settings.private_key_path)
    setEnabled(settings.enabled)
  }, [settings])

  const updateMutation = useMutation({
    mutationFn: updateSSLSettings,
    onSuccess: () => {
      toast.success(
        'تنظیمات SSL ذخیره شد. برای اعمال شدن، سرویس پنل را راه‌اندازی مجدد کنید.'
      )
      queryClient.invalidateQueries({ queryKey: ['ssl_settings'] })
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ذخیره تنظیمات SSL ناموفق بود.')),
  })

  const handleSave = () => {
    updateMutation.mutate({
      domain,
      certificate_path: certificatePath,
      private_key_path: privateKeyPath,
      enabled,
    })
  }

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconLock className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>SSL</CardTitle>
          <CardDescription>
            سرویس‌دهی پنل از طریق HTTPS با استفاده از گواهی‌ای که خودتان از
            قبل دریافت کرده‌اید (مثلاً از طریق Certbot روی همین سرور). پنل
            فقط این دو فایل را می‌خواند -- گواهی صادر یا تمدید نمی‌کند.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-6'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : (
          <>
            <div className='flex items-center justify-between rounded-lg border p-3'>
              <div>
                <Label htmlFor='ssl-enabled' className='text-sm font-medium'>
                  فعال‌سازی SSL
                </Label>
                <p className='text-muted-foreground text-xs'>
                  در حالت روشن، پنل به‌جای HTTP ساده، با HTTPS و با استفاده
                  از گواهی و کلید زیر راه‌اندازی می‌شود.
                </p>
              </div>
              <Switch
                id='ssl-enabled'
                checked={enabled}
                onCheckedChange={setEnabled}
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='ssl-domain'>دامنه</Label>
              <Input
                id='ssl-domain'
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder='pro.example.com'
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='ssl-cert-path'>مسیر گواهی (Certificate Path)</Label>
              <Input
                id='ssl-cert-path'
                value={certificatePath}
                onChange={(e) => setCertificatePath(e.target.value)}
                placeholder='/etc/letsencrypt/live/pro.example.com/fullchain.pem'
                className='font-mono text-xs'
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='ssl-key-path'>مسیر کلید خصوصی (Private Key Path)</Label>
              <Input
                id='ssl-key-path'
                value={privateKeyPath}
                onChange={(e) => setPrivateKeyPath(e.target.value)}
                placeholder='/etc/letsencrypt/live/pro.example.com/privkey.pem'
                className='font-mono text-xs'
              />
            </div>

            <Button onClick={handleSave} disabled={updateMutation.isPending}>
              {updateMutation.isPending ? 'در حال ذخیره...' : 'ذخیره تنظیمات SSL'}
            </Button>

            <Alert>
              <AlertTitle>نیاز به راه‌اندازی مجدد</AlertTitle>
              <AlertDescription>
                ذخیره کردن فقط این تنظیمات را اعتبارسنجی و ثبت می‌کند -- سرور
                در حال اجرا تا زمان راه‌اندازی مجدد فرآیند پنل، همچنان با
                پروتکل فعلی خود کار می‌کند. پیش از فعال‌سازی و راه‌اندازی
                مجدد، مطمئن شوید فایل‌های گواهی/کلید واقعاً برای کاربر
                فرآیند پنل قابل خواندن هستند، وگرنه پنل اجرا نخواهد شد.
              </AlertDescription>
            </Alert>
          </>
        )}
      </CardContent>
    </Card>
  )
}
