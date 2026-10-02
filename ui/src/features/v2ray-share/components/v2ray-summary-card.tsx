'use client'

import { V2RayPackageShareDetails } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { ClipboardCopyIcon, GaugeIcon } from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'

interface Props {
  details: V2RayPackageShareDetails
}

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

// The final box on the share page (after one V2RayLocationCard per
// authorized panel): package-wide used/remaining volume, online status,
// and the combined subscription link/QR that v2rayNG/v2box/etc actually
// subscribe to -- distinct from any single panel's own config_link shown
// on its own V2RayLocationCard above.
export default function V2RaySummaryCard({ details }: Props) {
  const remainingBytes = Math.max(
    0,
    details.total_volume_bytes - details.used_bytes
  )

  const handleCopySubscriptionUrl = async () => {
    const succeeded = await copyToClipboard(details.subscription_url)
    if (succeeded) {
      toast.success('لینک اشتراک در کلیپ‌بورد کپی شد', {
        duration: 5000,
      })
    } else {
      toast.error('کپی در کلیپ‌بورد ناموفق بود', { duration: 5000 })
    }
  }

  return (
    <Card className='gap-3'>
      <CardHeader>
        <CardTitle>خلاصه</CardTitle>
      </CardHeader>
      <CardContent className='space-y-4 text-sm'>
        <div className='text-foreground flex items-center justify-between font-semibold'>
          <span className='flex items-center gap-2'>
            <GaugeIcon className='h-4 w-4' />
            مجموع مصرف
          </span>
          <span>
            {bytesToGb(details.used_bytes)} GB
            {details.total_volume_bytes
              ? ` / ${bytesToGb(details.total_volume_bytes)} GB`
              : ''}
            {details.usage_percent != null ? ` (${details.usage_percent}%)` : ''}
          </span>
        </div>

        {details.total_volume_bytes ? (
          <Progress value={Number(details.usage_percent ?? 0)} className='h-3' />
        ) : null}

        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground'>باقی‌مانده</span>
          <span>{bytesToGb(remainingBytes)} GB</span>
        </div>

        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground'>وضعیت</span>
          <div className='flex items-center gap-2 text-sm font-medium'>
            <span
              className={
                details.is_online
                  ? 'h-2.5 w-2.5 rounded-full bg-green-500'
                  : 'h-2.5 w-2.5 rounded-full bg-red-500'
              }
            />
            <span className={details.is_online ? 'text-green-500' : 'text-red-500'}>
              {details.is_online ? 'آنلاین' : 'آفلاین'}
            </span>
          </div>
        </div>

        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground'>زمان انقضا</span>
          <span>{details.expire_at ?? 'هرگز'}</span>
        </div>

        <div className='space-y-2'>
          <span className='text-muted-foreground'>لینک اشتراک (v2rayNG / v2box)</span>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='min-w-0 flex-1 truncate font-mono text-xs'>
              {details.subscription_url}
            </span>
            <Button
              variant='ghost'
              size='icon'
              className='h-7 w-7 shrink-0'
              onClick={handleCopySubscriptionUrl}
            >
              <ClipboardCopyIcon className='h-3.5 w-3.5' />
            </Button>
          </div>
          <p className='text-muted-foreground text-xs'>
            این لینک را به‌عنوان اشتراک در اپلیکیشن V2Ray خود وارد کنید تا همه‌ی سرورهای بالا را یکجا دریافت کنید.
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
