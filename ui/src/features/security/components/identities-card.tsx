import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { IconSearch, IconUsers } from '@tabler/icons-react'
import { fetchSecurityIdentities } from '@/api/security.ts'
import { formatDateTimeFa, protocolLabelFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { IdentityHistoryDialog } from '@/features/security/components/identity-history-dialog.tsx'

type StatusFilter = 'all' | 'online' | 'offline'

export function IdentitiesCard() {
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [selected, setSelected] = useState<{ protocol: string; identity: string } | null>(null)

  const onlineOnly =
    statusFilter === 'all' ? undefined : statusFilter === 'online'

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['security_identities', debouncedSearch, statusFilter],
    queryFn: () => fetchSecurityIdentities(debouncedSearch, onlineOnly),
  })

  return (
    <Card className='animate-in fade-in slide-in-from-bottom-2 duration-500'>
      <CardHeader className='flex flex-col gap-3 space-y-0 sm:flex-row sm:items-center'>
        <div className='flex flex-1 items-center gap-2'>
          <IconUsers className='h-5 w-5' />
          <CardTitle className='text-lg'>کاربران</CardTitle>
        </div>
        <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as StatusFilter)}>
          <SelectTrigger className='w-full sm:w-36'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>همه</SelectItem>
            <SelectItem value='online'>فقط متصل</SelectItem>
            <SelectItem value='offline'>فقط قطع</SelectItem>
          </SelectContent>
        </Select>
        <div className='relative w-full sm:w-64'>
          <IconSearch className='text-muted-foreground pointer-events-none absolute end-2.5 top-1/2 size-4 -translate-y-1/2' />
          <Input
            placeholder='جستجو بر اساس یوزر، شهر، کشور یا ISP...'
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setDebouncedSearch(e.target.value)
            }}
            className='pe-9'
          />
        </div>
      </CardHeader>
      <CardContent>
        {isLoading || isFetching ? (
          <Skeleton className='h-64 w-full rounded-lg' />
        ) : !data || data.length === 0 ? (
          <EmptyState message='هیچ کاربری یافت نشد.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-start'>کاربر</TableHead>
                  <TableHead className='text-start'>پروتکل</TableHead>
                  <TableHead className='text-start'>آخرین آی‌پی</TableHead>
                  <TableHead className='text-start'>موقعیت</TableHead>
                  <TableHead className='text-start'>اپراتور</TableHead>
                  <TableHead className='text-start'>آخرین اتصال</TableHead>
                  <TableHead className='text-start'>وضعیت</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((identity) => (
                  <TableRow
                    key={`${identity.protocol}-${identity.identity}`}
                    className='hover:bg-muted/50 cursor-pointer'
                    onClick={() =>
                      setSelected({
                        protocol: identity.protocol,
                        identity: identity.identity,
                      })
                    }
                  >
                    <TableCell className='font-medium'>{identity.identity}</TableCell>
                    <TableCell>{protocolLabelFa(identity.protocol)}</TableCell>
                    <TableCell className='font-mono text-xs'>
                      {identity.last_ip_address}
                    </TableCell>
                    <TableCell>
                      {[identity.last_city, identity.last_country]
                        .filter(Boolean)
                        .join('، ') || '—'}
                    </TableCell>
                    <TableCell>{identity.last_isp || '—'}</TableCell>
                    <TableCell>{formatDateTimeFa(identity.last_connected_at)}</TableCell>
                    <TableCell>
                      {identity.is_currently_open ? (
                        <Badge className='bg-green-500 text-white hover:bg-green-500'>
                          متصل
                        </Badge>
                      ) : (
                        <Badge variant='secondary'>قطع</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <IdentityHistoryDialog
        open={!!selected}
        onOpenChange={(open) => !open && setSelected(null)}
        protocol={selected?.protocol ?? null}
        identity={selected?.identity ?? null}
      />
    </Card>
  )
}
