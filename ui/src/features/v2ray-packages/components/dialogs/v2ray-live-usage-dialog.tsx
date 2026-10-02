import { useEffect } from 'react'
import { V2RayPackage } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { RefreshCwIcon } from 'lucide-react'
import { useV2RayLiveUsageMutation } from '@/hooks/v2ray/useV2RayLiveUsageMutation.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

type Props = {
  open: boolean
  onOpenChange: (state: boolean) => void
  currentRow: V2RayPackage
}

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

// On-demand confirmation only -- fetches x-ui LIVE for every location of
// this package the moment the dialog opens (see
// V2RayPackageService.GetLiveUsage's own doc comment), never on a poll.
// The ordinary package list/table elsewhere in the app must keep showing
// the periodic sync job's cached figure; this dialog exists specifically
// so an admin/reseller can confirm a customer's CURRENT usage on demand
// (e.g. mid support call) without waiting for the next scheduled tick.
export function V2RayLiveUsageDialog({ open, onOpenChange, currentRow }: Props) {
  const { mutate, data, isPending, isError, error, reset } =
    useV2RayLiveUsageMutation()

  useEffect(() => {
    if (open) {
      mutate(currentRow.id)
    } else {
      reset()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, currentRow.id])

  const handleRefresh = () => mutate(currentRow.id)

  const usagePercent =
    data && data.total_volume_bytes > 0
      ? Math.min(100, (data.total_used_bytes / data.total_volume_bytes) * 100)
      : 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle className='flex flex-wrap items-center justify-between gap-2 pr-6'>
            <span>مصرف لحظه‌ای</span>
            <Button
              variant='outline'
              size='sm'
              onClick={handleRefresh}
              disabled={isPending}
            >
              <RefreshCwIcon
                className={`mr-2 h-3.5 w-3.5 ${isPending ? 'animate-spin' : ''}`}
              />
              به‌روزرسانی
            </Button>
          </DialogTitle>
          <DialogDescription>
            همین الان به‌صورت لحظه‌ای از هر پنل دریافت شده -- نه مقدار
            ذخیره‌شده‌ای که در لیست بسته‌ها نمایش داده می‌شود.
          </DialogDescription>
        </DialogHeader>

        {isPending && !data ? (
          <div className='space-y-3'>
            <Skeleton className='h-4 w-full' />
            <Skeleton className='h-24 w-full' />
          </div>
        ) : isError ? (
          <p className='text-destructive text-sm'>
            دریافت مصرف لحظه‌ای ناموفق بود:{' '}
            {error instanceof Error ? error.message : 'خطای ناشناخته'}
          </p>
        ) : data ? (
          <div className='space-y-4'>
            <div className='space-y-2'>
              <div className='text-foreground flex items-center justify-between text-sm font-semibold'>
                <span>مجموع</span>
                <span>
                  {bytesToGb(data.total_used_bytes)} GB
                  {data.total_volume_bytes
                    ? ` / ${bytesToGb(data.total_volume_bytes)} GB`
                    : ''}
                </span>
              </div>
              {data.total_volume_bytes > 0 && (
                <Progress value={usagePercent} className='h-2.5' />
              )}
            </div>

            <div className='overflow-x-auto rounded-md border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>پنل</TableHead>
                    <TableHead>وضعیت</TableHead>
                    <TableHead className='text-right'>آپلود</TableHead>
                    <TableHead className='text-right'>دانلود</TableHead>
                    <TableHead className='text-right'>مجموع</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.locations.map((loc) => (
                    <TableRow key={loc.panel_id}>
                      <TableCell className='font-medium'>
                        {loc.panel_name}
                      </TableCell>
                      <TableCell>
                        {loc.error ? (
                          <Badge variant='destructive'>خطا</Badge>
                        ) : (
                          <Badge
                            variant={loc.enabled ? 'default' : 'secondary'}
                          >
                            {loc.enabled ? 'فعال' : 'غیرفعال'}
                          </Badge>
                        )}
                      </TableCell>
                      {loc.error ? (
                        <TableCell
                          colSpan={3}
                          className='text-destructive text-right text-xs'
                        >
                          {loc.error}
                        </TableCell>
                      ) : (
                        <>
                          <TableCell className='text-right'>
                            {bytesToGb(loc.up_bytes)} GB
                          </TableCell>
                          <TableCell className='text-right'>
                            {bytesToGb(loc.down_bytes)} GB
                          </TableCell>
                          <TableCell className='text-right font-medium'>
                            {bytesToGb(loc.total_bytes)} GB
                          </TableCell>
                        </>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
