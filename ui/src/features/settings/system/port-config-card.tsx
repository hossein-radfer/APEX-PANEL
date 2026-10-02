import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AxiosError } from 'axios'
import { toast } from 'sonner'
import { IconPlugConnected } from '@tabler/icons-react'
import { fetchPortConfig, updatePortConfig } from '@/api/system-config.ts'
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
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

function backendErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const message = err.response?.data?.message
    if (typeof message === 'string' && message.length > 0) return message
  }
  return fallback
}

export function PortConfigCard() {
  const queryClient = useQueryClient()
  const { data: portConfig, isLoading } = useQuery({
    queryKey: ['port_config'],
    queryFn: fetchPortConfig,
  })

  const [portInput, setPortInput] = useState('')

  useEffect(() => {
    if (!portConfig) return
    setPortInput(String(portConfig.pending_port ?? portConfig.current_port))
  }, [portConfig])

  const updateMutation = useMutation({
    mutationFn: updatePortConfig,
    onSuccess: () => {
      toast.success(
        'پورت ذخیره شد. برای اعمال آن، سرویس پنل را ری‌استارت کنید.'
      )
      queryClient.invalidateQueries({ queryKey: ['port_config'] })
    },
    onError: (err) =>
      toast.error(backendErrorMessage(err, 'ذخیره‌ی پورت ناموفق بود.')),
  })

  const handleSave = () => {
    const port = Number.parseInt(portInput, 10)
    if (!Number.isFinite(port) || port < 1 || port > 65535) {
      toast.error('یک پورت معتبر بین ۱ تا ۶۵۵۳۵ وارد کنید.')
      return
    }
    updateMutation.mutate({ port })
  }

  const parsedInput = Number.parseInt(portInput, 10)
  const hasPendingChange =
    portConfig != null &&
    Number.isFinite(parsedInput) &&
    parsedInput !== portConfig.current_port

  return (
    <Card>
      <CardHeader className='flex flex-row items-center gap-3 pb-2'>
        <IconPlugConnected className='h-5 w-5' />
        <div>
          <CardTitle className='text-lg'>پورت پنل</CardTitle>
          <CardDescription>
            پورت TCP که پنل وب روی آن گوش می‌دهد. پیش‌فرض 3000 است.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className='space-y-4'>
        {isLoading ? (
          <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
        ) : (
          <>
            <div className='space-y-2'>
              <Label htmlFor='panel-port'>پورت</Label>
              <div className='flex max-w-xs items-center gap-2'>
                <Input
                  id='panel-port'
                  type='number'
                  min={1}
                  max={65535}
                  value={portInput}
                  onChange={(e) => setPortInput(e.target.value)}
                />
                <Button
                  onClick={handleSave}
                  disabled={updateMutation.isPending || !hasPendingChange}
                >
                  {updateMutation.isPending ? 'در حال ذخیره...' : 'ذخیره'}
                </Button>
              </div>
              {portConfig && (
                <p className='text-muted-foreground text-xs'>
                  در حال حاضر روی پورت {portConfig.current_port} در حال اجراست.
                </p>
              )}
            </div>

            <Alert>
              <AlertTitle>نیاز به ری‌استارت</AlertTitle>
              <AlertDescription>
                تغییر پورت فقط پس از ری‌استارت شدن فرآیند پنل اعمال می‌شود
                (سرور در حال اجرا تا آن زمان به گوش دادن روی پورت فعلی خود
                ادامه می‌دهد). اگر از راه دور به پنل دسترسی دارید، مطمئن
                شوید پورت جدید{' '}
                <span className='font-medium'>پیش از</span> ری‌استارت کردن
                در دسترس است، وگرنه ممکن است تا زمانی که بتوانید از راه
                دیگری (مثلاً SSH) به سرور دسترسی پیدا کنید و آن را برگردانید،
                دسترسی‌تان قطع شود.
              </AlertDescription>
            </Alert>

            {portConfig?.pending_port != null && (
              <Alert>
                <AlertTitle>تغییر در انتظار است</AlertTitle>
                <AlertDescription>
                  پورت {portConfig.pending_port} ذخیره شده و در ری‌استارت
                  بعدی پنل استفاده خواهد شد.
                </AlertDescription>
              </Alert>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
