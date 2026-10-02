import { DNSAccount, DNSAccountStatus } from '@/schema/dns-account.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { cn } from '@/lib/utils'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { ResellerDNSTableRowActions } from '@/features/reseller-dns/components/reseller-dns-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

const statusColor: Record<DNSAccountStatus, 'green' | 'yellow' | 'red'> = {
  active: 'green',
  suspended: 'yellow',
  expired: 'red',
}

// Mirrors dnsAccountsColumns, except the row-actions cell uses
// ResellerDNSTableRowActions (wired to the admin "Resellers DNS" page
// context) instead of the normal DNSAccountsTableRowActions -- same
// convention as resellerV2RayColumns.
export const resellerDNSColumns: ColumnDef<DNSAccount>[] = [
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
    accessorKey: 'panel_name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پنل DNS' />
    ),
    cell: ({ row }) => <div>{row.getValue('panel_name')}</div>,
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
      const account = row.original
      return (
        <div className='min-w-[140px] text-nowrap'>
          <div className='flex flex-col leading-tight'>
            <span className='text-foreground text-sm font-medium'>
              {bytesToGb(account.used_bytes)} GB
              <span className='text-muted-foreground text-sm'>
                {' / '}
                {account.total_volume_bytes
                  ? `${bytesToGb(account.total_volume_bytes)} GB`
                  : 'نامحدود'}
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
    cell: ResellerDNSTableRowActions,
  },
]
