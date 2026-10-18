import { ColumnDef } from '@tanstack/react-table'
import { Interface } from '@/schema/interfaces.ts'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useUpdateInterfaceStatusMutation } from '@/hooks/interfaces/useUpdateInterfaceStatusMutation.ts'
import { Switch } from '@/components/ui/switch.tsx'
import LongText from '@/components/long-text'
import { InterfacesTableRowActions } from '@/features/interfaces/components/interfaces-table-row-actions.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { ServerBadge } from '@/features/shared-components/server-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'

export const interfacesColumns: ColumnDef<Interface>[] = [
  {
    id: 'is_active',
    cell: ({ row }) => {
      const iface = row.original
      const updateInterfaceStatusMutation = useUpdateInterfaceStatusMutation()

      const handleToggle = () => {
        updateInterfaceStatusMutation.mutate(iface.id, {
          onSuccess: () => {
            toast.success(
              `اینترفیس با موفقیت ${!iface.disabled ? 'غیرفعال' : 'فعال'} شد`,
              {
                duration: 5000,
              }
            )
          },
        })
      }

      return (
        <div className='flex items-center justify-center'>
          <Switch
            id={`status-${iface.id}`}
            checked={!iface.disabled}
            onCheckedChange={handleToggle}
            disabled={updateInterfaceStatusMutation.isPending}
          />
        </div>
      )
    },
    enableSorting: false,
  },
  {
    id: 'name',
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام' />
    ),
    cell: ({ row, table }) => {
      const { name, server_id, server_name } = row.original
      // The server badge is only shown once the loaded rows actually span
      // more than one server -- in the common single-server case (today's
      // only real deployment) it would be pure visual noise, matching
      // InterfaceSelect's own identical convention.
      const allRows = table.getCoreRowModel().rows.map((r) => r.original)
      const showServerBadge =
        new Set(allRows.map((r) => r.server_id)).size > 1
      return (
        <div className='flex w-fit flex-col gap-0.5 text-nowrap'>
          <span>{name}</span>
          {showServerBadge && (
            <ServerBadge serverId={server_id} serverName={server_name} />
          )}
        </div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'comment',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='توضیحات' />
    ),
    cell: ({ row }) => (
      <LongText className='max-w-36'>
        {row.getValue('comment') ?? (
          <span className='text-muted-foreground'>ندارد</span>
        )}
      </LongText>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
    enableHiding: false,
  },
  {
    accessorKey: 'listen_port',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پورت شنود' />
    ),
    cell: ({ row }) => <div>{row.getValue('listen_port')}</div>,
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
    cell: ({ row }) =>
      row.original.is_running ? (
        <ColoredBadge color='green' text='در حال اجرا' />
      ) : (
        <ColoredBadge color='red' text='غیرفعال' />
      ),
    filterFn: (row, columnId, filterValue: string[]) => {
      const cellValue = row.getValue(columnId) as string[]
      return filterValue.some((val) => cellValue.includes(val))
    },
    meta: {
      className: cn('border-l border-r'),
    },
    enableHiding: false,
    enableSorting: false,
  },
  {
    id: 'actions',
    cell: InterfacesTableRowActions,
  },
]
