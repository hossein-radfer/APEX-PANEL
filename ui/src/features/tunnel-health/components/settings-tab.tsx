import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  fetchTunnelAiSettings,
  updateTunnelAiSettings,
} from '@/api/tunnel-health.ts'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { IconAlertTriangle } from '@tabler/icons-react'

// SettingsTab is the admin's own "گزینه آزمایشی" control (current
// explicit request): dry_run=true (the default) means the decision
// engine computes and LOGS every remediation command it would send, but
// never actually calls the mikrotik adaptor -- see
// TunnelHealthService.IsDryRun on the backend. Turning it off is what
// lets Level 1/2 actions actually run against RouterOS.
export function SettingsTab() {
  const queryClient = useQueryClient()
  const { data: settings, isLoading } = useQuery({
    queryKey: ['tunnel_ai_settings'],
    queryFn: () => fetchTunnelAiSettings(),
  })

  const handleDryRunChange = async (checked: boolean) => {
    try {
      await updateTunnelAiSettings({ dry_run: checked })
      queryClient.invalidateQueries({ queryKey: ['tunnel_ai_settings'] })
      toast.success(
        checked
          ? 'حالت آزمایشی فعال شد -- دستورات فقط نمایش داده می‌شوند'
          : 'حالت آزمایشی غیرفعال شد -- دستورات واقعاً روی روتر اجرا می‌شوند'
      )
    } catch {
      toast.error('خطا در ذخیره تنظیمات')
    }
  }

  const handleEmergencyStopChange = async (checked: boolean) => {
    try {
      await updateTunnelAiSettings({ emergency_stop: checked })
      queryClient.invalidateQueries({ queryKey: ['tunnel_ai_settings'] })
      toast.success(
        checked
          ? 'توقف اضطراری فعال شد -- هیچ اقدامی انجام نمی‌شود'
          : 'توقف اضطراری غیرفعال شد'
      )
    } catch {
      toast.error('خطا در ذخیره تنظیمات')
    }
  }

  if (isLoading || !settings) {
    return <Skeleton className='h-40 w-full rounded-lg' />
  }

  return (
    <div className='space-y-4'>
      {!settings.dry_run && (
        <Alert variant='destructive'>
          <IconAlertTriangle className='h-4 w-4' />
          <AlertTitle>حالت آزمایشی غیرفعال است</AlertTitle>
          <AlertDescription>
            موتور تصمیم‌گیری اکنون می‌تواند دستورات واقعی (سطح ۱: خاموش و
            روشن کردن اینترفیس) را مستقیماً روی روتر اجرا کند.
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>تنظیمات مدیریت هوشمند تانل</CardTitle>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='flex items-center justify-between gap-4'>
            <div className='space-y-0.5'>
              <Label htmlFor='dry-run-switch'>حالت آزمایشی (Dry-run)</Label>
              <p className='text-muted-foreground text-sm'>
                وقتی فعال است، موتور فقط دستوراتی که می‌خواهد اجرا کند را
                در تاریخچه ثبت و نمایش می‌دهد؛ هیچ تغییری روی میکروتیک
                اعمال نمی‌شود. برای فعال‌سازی اجرای واقعی، این گزینه را
                خاموش کنید.
              </p>
            </div>
            <Switch
              id='dry-run-switch'
              checked={settings.dry_run}
              onCheckedChange={handleDryRunChange}
            />
          </div>

          <div className='flex items-center justify-between gap-4 border-t pt-6'>
            <div className='space-y-0.5'>
              <Label htmlFor='emergency-stop-switch'>توقف اضطراری</Label>
              <p className='text-muted-foreground text-sm'>
                وقتی فعال است، موتور صرفاً مشاهده می‌کند و هیچ اقدام
                درمانی (شبیه‌سازی‌شده یا واقعی) انجام نمی‌دهد -- همیشه در
                دسترس، مستقل از حالت آزمایشی.
              </p>
            </div>
            <Switch
              id='emergency-stop-switch'
              checked={settings.emergency_stop}
              onCheckedChange={handleEmergencyStopChange}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
