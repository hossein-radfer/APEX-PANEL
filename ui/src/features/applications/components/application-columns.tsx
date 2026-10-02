import { Application } from '@/schema/application.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { cn } from '@/lib/utils'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { ApplicationTableRowActions } from '@/features/applications/components/application-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

const statusColor: Record<Application['status'], 'green' | 'yellow' | 'red'> = {
  active: 'green',
  suspended: 'yellow',
  expired: 'red',
}

export const applicationColumns: ColumnDef<Application>[] = [
  {
    id: 'name',
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام' />
    ),
    cell: ({ row }) => (
      <div className='w-fit text-nowrap font-medium'>{row.original.name}</div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    id: 'app_username',
    accessorKey: 'app_username',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='یوزرنیم اپ' />
    ),
    cell: ({ row }) => (
      <div className='w-fit font-mono text-xs'>{row.original.app_username}</div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'total_volume_bytes',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='حجم' />
    ),
    cell: ({ row }) => {
      const app = row.original
      return (
        <div className='min-w-[140px] text-nowrap'>
          <span className='text-foreground text-sm font-medium'>
            {bytesToGb(app.used_bytes)} GB
            <span className='text-muted-foreground text-sm'>
              {' / '}
              {bytesToGb(app.total_volume_bytes)} GB
            </span>
          </span>
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
      <DataTableColumnHeader column={column} title='مدت' />
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
      return <div className='w-fit text-nowrap'>{expire_at ? expire_at : 'نامحدود'}</div>
    },
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'max_online_users',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='کاربر آنلاین مجاز' />
    ),
    cell: ({ row }) => <div>{row.getValue('max_online_users')}</div>,
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
      const { status, disabled } = row.original
      if (disabled) {
        return <ColoredBadge color='red' text='غیرفعال' />
      }
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
    cell: ApplicationTableRowActions,
  },
]
