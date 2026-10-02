'use client'

import { AxiosError } from 'axios'
import { useSearch } from '@tanstack/react-router'
import { IconRoute } from '@tabler/icons-react'
import { Loader2 } from 'lucide-react'
import { useV2RayPackageShareDetailsQuery } from '@/hooks/v2ray/useV2RayPackageShareDetailsQuery.ts'
import NotFoundError from '@/features/errors/not-found-error.tsx'
import { v2rayShareFaq } from '@/features/share/lib/faq-content.ts'
import { SmartFaq } from '@/features/shared-components/smart-faq.tsx'
import V2RayLocationCard from '@/features/v2ray-share/components/v2ray-location-card.tsx'
import V2RaySummaryCard from '@/features/v2ray-share/components/v2ray-summary-card.tsx'

export default function V2RayPackageShare() {
  const { shareId } = useSearch({ from: '/v2ray-share' })

  const {
    data: details,
    error: detailsError,
    isLoading: detailsLoading,
  } = useV2RayPackageShareDetailsQuery(shareId)

  if (detailsError && (detailsError as AxiosError)?.response?.status === 404) {
    return <NotFoundError />
  }

  return (
    <div className='mx-auto max-w-5xl space-y-4 p-4 sm:p-6'>
      <div className='space-y-3 text-center'>
        <div className='text-primary flex items-center justify-center gap-2'>
          <IconRoute className='h-6 w-6 shrink-0' />
          <h1 className='text-2xl font-bold sm:text-3xl'>ApexPanel</h1>
          <IconRoute className='h-6 w-6 shrink-0' />
        </div>
        <h2 className='text-foreground text-lg font-semibold sm:text-xl'>
          {details?.customer_label
            ? `سلام 👋 ${details.customer_label}`
            : 'پکیج V2Ray شما'}
        </h2>
        <p className='text-muted-foreground text-sm'>
          کد QR هر سرور را با اپلیکیشن V2Ray خود اسکن کنید، یا لینک اشتراک
          ترکیبی پایین صفحه را کپی کنید تا همه‌ی سرورها یکجا اضافه شوند.
        </p>
      </div>

      {detailsLoading || !details ? (
        <div className='flex h-64 items-center justify-center'>
          <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
        </div>
      ) : (
        <div className='grid gap-4 sm:gap-6 md:grid-cols-2 xl:grid-cols-3'>
          {details.locations.map((location, i) => (
            <V2RayLocationCard
              key={`${location.title}-${i}`}
              location={location}
              expireAt={details.expire_at}
            />
          ))}
          <V2RaySummaryCard details={details} />
        </div>
      )}

      <div className='mx-auto max-w-2xl'>
        <SmartFaq items={v2rayShareFaq} />
      </div>
    </div>
  )
}
