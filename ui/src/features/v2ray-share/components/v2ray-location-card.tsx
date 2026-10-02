'use client'

import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { V2RayShareLocationUsage } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { ClipboardCopyIcon } from 'lucide-react'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/clipboard.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface Props {
  location: V2RayShareLocationUsage
  expireAt: string | null | undefined
}

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

// One box per registered/authorized x-ui panel on the share page: the
// panel's title centered at top, its own single raw config link (with
// copy), the shared package expiry, that location's own usage, and a QR
// code of that one link -- per the exact per-panel layout requested,
// distinct from the final summary box (V2RaySummaryCard) which covers the
// package as a whole.
export default function V2RayLocationCard({ location, expireAt }: Props) {
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null)

  useEffect(() => {
    if (!location.config_link) {
      setQrDataUrl(null)
      return
    }

    let cancelled = false
    QRCode.toDataURL(location.config_link, { width: 220, margin: 1 })
      .then((url) => {
        if (!cancelled) setQrDataUrl(url)
      })
      .catch(() => {
        if (!cancelled) setQrDataUrl(null)
      })

    return () => {
      cancelled = true
    }
  }, [location.config_link])

  const handleCopyLink = async () => {
    if (!location.config_link) return
    const succeeded = await copyToClipboard(location.config_link)
    if (succeeded) {
      toast.success('لینک در کلیپ‌بورد کپی شد', { duration: 5000 })
    } else {
      toast.error('کپی در کلیپ‌بورد ناموفق بود', { duration: 5000 })
    }
  }

  return (
    <Card className='gap-3'>
      <CardHeader>
        <CardTitle className='flex items-center justify-center gap-2 text-center'>
          {location.title}
          <Badge
            variant={location.is_online ? 'default' : 'secondary'}
            className={
              location.is_online
                ? 'bg-sky-500 text-white hover:bg-sky-500 dark:bg-sky-400'
                : ''
            }
          >
            {location.is_online ? 'آنلاین' : 'آفلاین'}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-4 text-sm'>
        <div className='space-y-2'>
          <div className='flex min-w-0 items-center gap-2'>
            <span className='min-w-0 flex-1 truncate rounded-md border px-2 py-1.5 font-mono text-xs'>
              {location.config_link || '—'}
            </span>
            <Button
              variant='ghost'
              size='icon'
              className='h-8 w-8 shrink-0'
              disabled={!location.config_link}
              onClick={handleCopyLink}
            >
              <ClipboardCopyIcon className='h-3.5 w-3.5' />
            </Button>
          </div>
        </div>

        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground'>زمان انقضا</span>
          <span>{expireAt ?? 'هرگز'}</span>
        </div>

        <div className='flex items-center justify-between'>
          <span className='text-muted-foreground'>مصرف‌شده</span>
          <span>{bytesToGb(location.used_bytes)} GB</span>
        </div>

        <div className='flex items-center justify-center pt-2'>
          {!qrDataUrl ? (
            <Skeleton className='aspect-square w-full max-w-[220px] rounded-md' />
          ) : (
            <img
              src={qrDataUrl}
              alt={`${location.title} QR Code`}
              className='h-auto w-full max-w-[220px] rounded-md'
            />
          )}
        </div>
      </CardContent>
    </Card>
  )
}
