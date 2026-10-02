import { useQuery } from '@tanstack/react-query'
import { fetchLocationProfitability } from '@/api/accounting.ts'
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

// LocationProfitabilityTab rolls sales/costs up per (protocol, location)
// -- the admin's own explicit "هر لوکشین چقدر در یماد برام" requirement.
// A location only appears here once at least one sale or cost has been
// tagged with a matching location_key on both sides (see CostsTab's own
// "must match exactly" hint).
export function LocationProfitabilityTab() {
  const { data: rows = [], isLoading } = useQuery({
    queryKey: ['accounting_location_profitability'],
    queryFn: () => fetchLocationProfitability(),
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>سود هر لوکیشن (۳۰ روز اخیر)</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <EmptyState message='هیچ فروش یا هزینه‌ای با کلید لوکیشن مشخص در این بازه ثبت نشده است.' />
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>پروتکل</TableHead>
                  <TableHead>لوکیشن</TableHead>
                  <TableHead>درآمد</TableHead>
                  <TableHead>هزینه</TableHead>
                  <TableHead>سود</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows
                  .slice()
                  .sort((a, b) => b.profit_toman - a.profit_toman)
                  .map((row) => (
                    <TableRow key={`${row.protocol}-${row.location_key}`}>
                      <TableCell>
                        <Badge variant='secondary'>
                          {PROTOCOL_LABELS_FA[row.protocol] ?? row.protocol}
                        </Badge>
                      </TableCell>
                      <TableCell className='font-medium'>
                        {row.location_label || row.location_key}
                      </TableCell>
                      <TableCell className='tabular-nums'>
                        {formatCurrencyFa(row.income_toman)}
                      </TableCell>
                      <TableCell className='tabular-nums'>
                        {formatCurrencyFa(row.cost_toman)}
                      </TableCell>
                      <TableCell
                        className={`tabular-nums font-medium ${row.profit_toman >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'}`}
                      >
                        {formatCurrencyFa(row.profit_toman)}
                      </TableCell>
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
