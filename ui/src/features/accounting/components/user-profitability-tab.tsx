import { useQuery } from '@tanstack/react-query'
import { fetchUserProfitability } from '@/api/accounting.ts'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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

const PROTOCOL_LABELS_FA: Record<string, string> = {
  wireguard: 'وایرگارد',
  user_manager: 'یوزرمنجیر',
  v2ray: 'V2Ray',
}

// UserProfitabilityTab lists every sale with its own sale-price/cost/
// profit -- the admin's own explicit "برای هر یوزر بگم چقدر فروختم و سود
// رو محاسبه کنه" requirement. Reuses the last-30-days default the
// dashboard/summary endpoint also uses.
export function UserProfitabilityTab() {
  const { data: rows = [], isLoading } = useQuery({
    queryKey: ['accounting_user_profitability'],
    queryFn: () => fetchUserProfitability(),
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>سود هر کاربر (۳۰ روز اخیر)</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <EmptyState message='در این بازه هیچ فروشی ثبت نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>مشتری</TableHead>
                  <TableHead>پروتکل</TableHead>
                  <TableHead>لوکیشن</TableHead>
                  <TableHead>فروش</TableHead>
                  <TableHead>هزینه تمام‌شده</TableHead>
                  <TableHead>سود</TableHead>
                  <TableHead>تاریخ</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.payment_id}>
                    <TableCell className='font-medium'>{row.customer_label}</TableCell>
                    <TableCell>
                      {row.protocol ? (
                        <Badge variant='secondary'>
                          {PROTOCOL_LABELS_FA[row.protocol] ?? row.protocol}
                        </Badge>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell>{row.location_label || row.location_key || '—'}</TableCell>
                    <TableCell className='tabular-nums'>
                      {formatCurrencyFa(row.amount_toman)}
                    </TableCell>
                    <TableCell className='tabular-nums'>
                      {formatCurrencyFa(row.cost_toman)}
                    </TableCell>
                    <TableCell
                      className={`tabular-nums font-medium ${row.profit_toman >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}
                    >
                      {formatCurrencyFa(row.profit_toman)}
                    </TableCell>
                    <TableCell>{row.paid_at}</TableCell>
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
