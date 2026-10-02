'use client'

import { V2RayPackageShareDetails } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { ClipboardCopyIcon, GaugeIcon } from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'

interface Props {
  isLoading: boolean
  details: V2RayPackageShareDetails | undefined
}

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

export default function V2RayUsageCard({ isLoading, details }: Props) {
  const handleCopySubscriptionUrl = async () => {
    if (!details?.subscription_url) return
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
        <CardTitle>مصرف</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className='space-y-3'>
            <Skeleton className='h-5 w-full' />
            <Skeleton className='h-5 w-full' />
            <Skeleton className='h-5 w-2/3' />
          </div>
        ) : (
          <div className='space-y-4 text-sm'>
            <div className='text-foreground flex items-center justify-between font-semibold'>
              <span className='flex items-center gap-2'>
                <GaugeIcon className='h-4 w-4' />
                مجموع مصرف
              </span>
              <span>
                {details ? bytesToGb(details.used_bytes) : '0.00'} GB
                {details?.total_volume_bytes
                  ? ` / ${bytesToGb(details.total_volume_bytes)} GB`
                  : ''}
                {details?.usage_percent != null
                  ? ` (${details.usage_percent}%)`
                  : ''}
              </span>
            </div>

            {details?.total_volume_bytes ? (
              <Progress
                value={Number(details.usage_percent ?? 0)}
                className='h-3'
              />
            ) : null}

            {details?.locations && details.locations.length > 1 && (
              <div className='space-y-1.5 border-t pt-3'>
                <span className='text-muted-foreground text-xs font-medium'>
                  مصرف به تفکیک سرور
                </span>
                {details.locations.map((loc, i) => (
                  <div
                    key={`${loc.title}-${i}`}
                    className='flex items-center justify-between text-xs'
                  >
                    <span className='text-muted-foreground truncate'>
                      {loc.title}
                    </span>
                    <span>{bytesToGb(loc.used_bytes)} GB</span>
                  </div>
                ))}
              </div>
            )}

            <div className='flex items-center justify-between'>
              <span className='text-muted-foreground'>زمان انقضا</span>
              <span>{details?.expire_at ?? 'هرگز'}</span>
            </div>

            <div className='space-y-2'>
              <span className='text-muted-foreground'>لینک اشتراک</span>
              <div className='flex items-center gap-2'>
                <span className='truncate font-mono text-xs'>
                  {details?.subscription_url}
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
                این لینک را در اپلیکیشن V2Ray خود وارد کنید، یا QR Code را اسکن کنید.
              </p>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
