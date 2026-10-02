import { V2RayPackage, V2RayPackageStatus } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { cn } from '@/lib/utils'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { V2RayLocationStatus } from '@/features/v2ray-packages/components/v2ray-location-status.tsx'
import { ResellerV2RayTableRowActions } from '@/features/reseller-v2ray/components/reseller-v2ray-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

const statusColor: Record<V2RayPackageStatus, 'green' | 'yellow' | 'red'> = {
  active: 'green',
  suspended: 'yellow',
  expired: 'red',
}

// Mirrors v2rayColumns exactly, except the row-actions cell uses
// ResellerV2RayTableRowActions (wired to the admin "Resellers V2Ray" page
// context) instead of the normal V2RayTableRowActions -- same convention as
// resellerAccountColumns.
export const resellerV2RayColumns: ColumnDef<V2RayPackage>[] = [
  {
    id: 'customer_label',
    accessorKey: 'customer_label',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='برچسب مشتری' />
    ),
    cell: ({ row }) => (
      <div className='w-fit text-nowrap'>
        {row.original.customer_label ?? (
          <span className='text-muted-foreground'>ندارد</span>
        )}
      </div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'total_volume_bytes',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='ترافیک' />
    ),
    cell: ({ row }) => {
      const pkg = row.original
      return (
        <div className='min-w-[140px] text-nowrap'>
          <div className='flex flex-col leading-tight'>
            <span className='text-foreground text-sm font-medium'>
              {bytesToGb(pkg.used_bytes)} GB
              <span className='text-muted-foreground text-sm'>
                {' / '}
                {bytesToGb(pkg.total_volume_bytes)} GB
              </span>
            </span>
          </div>
        </div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'duration_days',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='مدت زمان' />
    ),
    cell: ({ row }) => <div>{row.getValue('duration_days')} روز</div>,
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'expire_at',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='انقضا' />
    ),
    cell: ({ row }) => {
      const { expire_at } = row.original
      return (
        <div className='w-fit text-nowrap'>{expire_at ? expire_at : 'هرگز'}</div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    id: 'locations',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='مکان‌ها' />
    ),
    cell: ({ row }) => <V2RayLocationStatus locations={row.original.locations} />,
    meta: {
      className: cn('border-l border-r'),
    },
    enableSorting: false,
  },
  {
    id: 'status',
    accessorKey: 'status',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='وضعیت' />
    ),
    cell: ({ row }) => {
      const { status } = row.original
      return <ColoredBadge color={statusColor[status]} text={status} />
    },
    filterFn: (row, columnId, filterValue: string[]) =>
      filterValue.includes(row.getValue(columnId)),
    meta: {
      className: cn('border-l border-r'),
    },
    enableSorting: false,
  },
  {
    id: 'actions',
    cell: ResellerV2RayTableRowActions,
  },
]
