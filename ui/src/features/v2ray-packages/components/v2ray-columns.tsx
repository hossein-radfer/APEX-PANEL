import { useState } from 'react'
import { V2RayPackage, V2RayPackageStatus } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { IconRestore } from '@tabler/icons-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useResetV2RayPackageUsageMutation } from '@/hooks/v2ray/useResetV2RayPackageUsageMutation.ts'
import { Button } from '@/components/ui/button.tsx'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { SimpleDialog } from '@/features/shared-components/table/dialogs/simple-dialog.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { V2RayLocationStatus } from '@/features/v2ray-packages/components/v2ray-location-status.tsx'
import { V2RayTableRowActions } from '@/features/v2ray-packages/components/v2ray-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'
import { useAuthStore } from '@/stores/authStore.ts'

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

const statusColor: Record<V2RayPackageStatus, 'green' | 'yellow' | 'red'> = {
  active: 'green',
  suspended: 'yellow',
  expired: 'red',
}

export const v2rayColumns: ColumnDef<V2RayPackage>[] = [
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
      const [dialogOpen, setDialogOpen] = useState(false)
      const resetUsageMutation = useResetV2RayPackageUsageMutation()
      const pkg = row.original
      // Admin-only action as of this fix: the backend now 403s a reseller
      // calling reset-usage (mirrors the identical WireGuard peer-columns
      // fix) -- hide the button for a reseller session.
      const role = useAuthStore((state) => state.auth.admin?.role)
      const isReseller = role === 'reseller'

      const usageDisplay = (
        <div className='flex flex-col leading-tight'>
          <span className='text-foreground text-sm font-medium'>
            {bytesToGb(pkg.used_bytes)} GB
            <span className='text-muted-foreground text-sm'>
              {' / '}
              {bytesToGb(pkg.total_volume_bytes)} GB
            </span>
          </span>
        </div>
      )

      if (isReseller) {
        return <div className='min-w-[140px] text-nowrap'>{usageDisplay}</div>
      }

      const handleResetUsage = async () => {
        resetUsageMutation.mutateAsync(pkg.id, {
          onSuccess: () => {
            setDialogOpen(false)
            toast.success('مصرف بسته با موفقیت بازنشانی شد', {
              duration: 5000,
            })
          },
          onError: () => {
            setDialogOpen(false)
          },
        })
      }

      return (
        <SimpleDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          title='بازنشانی مصرف بسته؟'
          description='این کار آمار مصرف بسته را بازنشانی می‌کند. آیا مطمئن هستید؟'
          actionText='تأیید بازنشانی'
          mutateAsync={handleResetUsage}
          trigger={
            <div className='min-w-[140px] text-nowrap'>
              <div className='flex items-center gap-3'>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant='ghost'
                      size='icon'
                      className='text-muted-foreground hover:text-foreground h-8 w-8'
                      aria-label='بازنشانی مصرف'
                    >
                      <IconRestore className='h-4 w-4' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side='top' align='center'>
                    <p>بازنشانی مصرف</p>
                  </TooltipContent>
                </Tooltip>
                {usageDisplay}
              </div>
            </div>
          }
        />
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
      <DataTableColumnHeader column={column} title='لوکیشن‌ها' />
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
    cell: V2RayTableRowActions,
  },
]
