import { useQuery } from '@tanstack/react-query'
import { fetchTunnelActionLogs } from '@/api/tunnel-health.ts'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

// ActionsTab is the admin's own requested view: exactly what the
// self-healing engine WOULD do (or did), one row per decision. In
// dry-run mode (the default) every row here has simulated=true and
// command_detail shows the literal RouterOS call that would have been
// sent -- nothing in this table implies it was actually executed unless
// the "شبیه‌سازی" badge says otherwise.
export function ActionsTab() {
  const { data: rows = [], isLoading } = useQuery({
    queryKey: ['tunnel_ai_actions'],
    queryFn: () => fetchTunnelActionLogs(200),
    refetchInterval: 30_000,
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>تاریخچه تصمیمات و اقدامات</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className='h-40 w-full rounded-lg' />
        ) : rows.length === 0 ? (
          <p className='text-muted-foreground py-8 text-center text-sm'>
            هنوز هیچ تصمیم درمانی ثبت نشده است.
          </p>
        ) : (
          <div className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>اینترفیس</TableHead>
                  <TableHead>سطح</TableHead>
                  <TableHead>حالت</TableHead>
                  <TableHead>توضیح</TableHead>
                  <TableHead>دستور دقیق</TableHead>
                  <TableHead>نتیجه</TableHead>
                  <TableHead>زمان</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className='font-medium'>
                      {row.interface_name}
                    </TableCell>
                    <TableCell>سطح {row.level}</TableCell>
                    <TableCell>
                      {row.simulated ? (
                        <Badge
                          variant='secondary'
                          className='bg-yellow-600/10 text-yellow-600 dark:bg-yellow-400/10 dark:text-yellow-400'
                        >
                          شبیه‌سازی
                        </Badge>
                      ) : (
                        <Badge
                          variant='secondary'
                          className='bg-green-600/10 text-green-600 dark:bg-green-400/10 dark:text-green-400'
                        >
                          اجرای واقعی
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className='max-w-xs text-sm'>
                      {row.command_description}
                    </TableCell>
                    <TableCell className='max-w-md font-mono text-xs whitespace-pre-wrap'>
                      {row.command_detail}
                    </TableCell>
                    <TableCell className='max-w-xs text-sm'>
                      {row.result}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {new Date(row.executed_at).toLocaleString('fa-IR')}
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
