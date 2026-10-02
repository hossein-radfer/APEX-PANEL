import { useQuery } from '@tanstack/react-query'
import { IconShieldExclamation } from '@tabler/icons-react'
import { fetchSecurityThreats } from '@/api/security.ts'
import { formatDateTimeFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// Same 10s cadence as EtherTrafficCard's own polling -- both cards read
// off the same background-collected IPConnectionLog data, so keeping them
// in sync avoids one looking "stale" relative to the other.
const REFRESH_INTERVAL_MS = 10_000

const KIND_LABELS_FA: Record<string, string> = {
  shared_account: 'اکانت اشتراکی',
  multi_country: 'اتصال چندکشوری',
  scanning: 'اسکن/تلاش مشکوک',
}

export function ThreatsCard() {
  const { data, isLoading } = useQuery({
    queryKey: ['security_threats'],
    queryFn: fetchSecurityThreats,
    refetchInterval: REFRESH_INTERVAL_MS,
  })

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='space-y-0'>
        <div className='flex items-center gap-2'>
          <IconShieldExclamation className='h-5 w-5' />
          <CardTitle className='text-lg'>تهدیدات</CardTitle>
        </div>
        <CardDescription>
          آی‌پی‌ها و کاربران مشکوک بر اساس الگوهای اتصال یک ساعت اخیر --
          اکانت‌های به‌اشتراک‌گذاشته‌شده، اتصال هم‌زمان از چند کشور، و
          منابع احتمالی اسکن/حمله.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-48 w-full rounded-lg' />
        ) : !data || data.length === 0 ? (
          <EmptyState message='در حال حاضر هیچ تهدیدی شناسایی نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-start'>نوع</TableHead>
                  <TableHead className='text-start'>موضوع</TableHead>
                  <TableHead className='text-start'>جزئیات</TableHead>
                  <TableHead className='text-start'>موقعیت</TableHead>
                  <TableHead className='text-start'>شدت</TableHead>
                  <TableHead className='text-start'>آخرین مشاهده</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((threat, i) => (
                  <TableRow key={`${threat.kind}-${threat.subject}-${i}`}>
                    <TableCell>{KIND_LABELS_FA[threat.kind] ?? threat.kind}</TableCell>
                    <TableCell className='font-mono text-xs'>{threat.subject}</TableCell>
                    <TableCell>{threat.detail}</TableCell>
                    <TableCell>
                      {[threat.city, threat.country].filter(Boolean).join('، ') || '—'}
                    </TableCell>
                    <TableCell>
                      {threat.severity === 'high' ? (
                        <Badge className='bg-red-500 text-white hover:bg-red-500'>
                          بالا
                        </Badge>
                      ) : (
                        <Badge className='bg-yellow-500 text-white hover:bg-yellow-500'>
                          متوسط
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>{formatDateTimeFa(threat.last_seen_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
