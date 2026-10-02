import type { ReactNode } from 'react'
import { formatCurrencyFa } from '@/features/reports/lib/format.ts'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { IconEdit, IconPackage, IconTrash } from '@tabler/icons-react'

// Shared table for the three billing traffic-package pages (plain,
// User Manager, V2Ray) -- these were three copy-pasted raw <table> blocks
// with identical columns (name/volume/price/status/actions) that only
// ever differed in which package type's fields they read, so collapsed
// into one generic component keyed off this minimal common shape instead
// of drifting further apart on future edits.
export interface TrafficPackageRow {
  id: number
  name: string
  traffic_bytes: number
  price_amount: number
  is_active: boolean
}

interface Props<T extends TrafficPackageRow> {
  packages: T[]
  isLoading: boolean
  bytesToGb: (bytes: number) => ReactNode
  onToggleActive: (pkg: T) => void
  onEdit: (pkg: T) => void
  onDelete: (pkg: T) => void
}

export function TrafficPackageTable<T extends TrafficPackageRow>({
  packages,
  isLoading,
  bytesToGb,
  onToggleActive,
  onEdit,
  onDelete,
}: Props<T>) {
  if (isLoading) {
    return <p className='text-muted-foreground'>در حال بارگذاری...</p>
  }

  if (packages.length === 0) {
    return (
      <EmptyState
        icon={<IconPackage className='size-10 opacity-60' />}
        message='هنوز بسته ترافیکی تعریف نشده است.'
      />
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow className='bg-muted/40'>
            <TableHead className='text-start'>نام</TableHead>
            <TableHead className='text-start'>حجم</TableHead>
            <TableHead className='text-start'>قیمت</TableHead>
            <TableHead className='text-start'>وضعیت</TableHead>
            <TableHead className='text-start'>عملیات</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {packages.map((pkg) => (
            <TableRow key={pkg.id}>
              <TableCell className='font-medium'>{pkg.name}</TableCell>
              <TableCell>{bytesToGb(pkg.traffic_bytes)} گیگابایت</TableCell>
              <TableCell className='tabular-nums'>
                {formatCurrencyFa(pkg.price_amount)}
              </TableCell>
              <TableCell>
                <Badge
                  variant={pkg.is_active ? 'default' : 'outline'}
                  className='cursor-pointer'
                  onClick={() => onToggleActive(pkg)}
                >
                  {pkg.is_active ? 'فعال' : 'غیرفعال'}
                </Badge>
              </TableCell>
              <TableCell>
                <div className='flex items-center gap-2'>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='h-8 w-8'
                    onClick={() => onEdit(pkg)}
                    aria-label='ویرایش بسته'
                  >
                    <IconEdit className='h-4 w-4' />
                  </Button>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='text-destructive h-8 w-8'
                    onClick={() => onDelete(pkg)}
                    aria-label='حذف بسته'
                  >
                    <IconTrash className='h-4 w-4' />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
