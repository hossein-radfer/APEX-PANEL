'use client'

import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface Props {
  isLoading: boolean
  value: string | undefined
}

// The V2Ray share endpoints only expose the subscription URL as text (see
// V2RayPackageShareDetailsResponse) -- there's no backend-rendered QR image
// endpoint the way peers/qrcode and user/qrcode have. Generated client-side
// instead, using the `qrcode` package (the only QR-capable dependency in
// this codebase; none existed before this feature).
export default function V2RayQRCodeCard({ isLoading, value }: Props) {
  const [dataUrl, setDataUrl] = useState<string | null>(null)

  useEffect(() => {
    if (!value) {
      setDataUrl(null)
      return
    }

    let cancelled = false
    QRCode.toDataURL(value, { width: 300, margin: 1 })
      .then((url) => {
        if (!cancelled) setDataUrl(url)
      })
      .catch(() => {
        if (!cancelled) setDataUrl(null)
      })

    return () => {
      cancelled = true
    }
  }, [value])

  return (
    <Card className='flex h-full flex-col'>
      <CardHeader>
        <CardTitle>کد QR</CardTitle>
      </CardHeader>

      <CardContent className='flex flex-1 items-center justify-center'>
        {isLoading || !dataUrl ? (
          <Skeleton className='h-[300px] w-[300px] rounded-md' />
        ) : (
          <img
            src={dataUrl}
            alt='Subscription QR Code'
            width={300}
            height={300}
            className='rounded-md'
          />
        )}
      </CardContent>
    </Card>
  )
}
