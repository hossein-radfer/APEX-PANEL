import { DNSPanel } from '@/schema/dns-panel.ts'
import { cn } from '@/lib/utils'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { DNSPanelsTableRowActions } from '@/features/dns-panels/components/dns-panels-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'

export const dnsPanelsColumns: ColumnDef<DNSPanel>[] = [
  {
    id: 'name',
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام' />
    ),
    cell: ({ row }) => (
      <div className='w-fit text-nowrap'>{row.original.name}</div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'sale_title',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='عنوان فروش' />
    ),
    cell: ({ row }) => <div>{row.getValue('sale_title')}</div>,
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'api_base_url',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='آدرس پایه API' />
    ),
    cell: ({ row }) => (
      <div className='max-w-64 truncate'>{row.getValue('api_base_url')}</div>
    ),
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
      const panel = row.original
      const badge =
        panel.status === 'active' ? (
          <ColoredBadge color='green' text='فعال' />
        ) : (
          <ColoredBadge color='red' text='خطا' />
        )

      if (panel.status === 'error' && panel.last_error) {
        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className='w-fit'>{badge}</div>
            </TooltipTrigger>
            <TooltipContent>
              <p className='max-w-64'>{panel.last_error}</p>
            </TooltipContent>
          </Tooltip>
        )
      }

      return badge
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
    cell: DNSPanelsTableRowActions,
  },
]
