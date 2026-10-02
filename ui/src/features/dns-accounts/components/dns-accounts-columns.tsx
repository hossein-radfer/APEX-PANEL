import { useState } from 'react'
import { DNSAccount, DNSAccountStatus } from '@/schema/dns-account.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { IconRestore } from '@tabler/icons-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useResetDNSAccountUsageMutation } from '@/hooks/dns-account/useResetDNSAccountUsageMutation.ts'
import { Button } from '@/components/ui/button.tsx'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { SimpleDialog } from '@/features/shared-components/table/dialogs/simple-dialog.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { DNSAccountsTableRowActions } from '@/features/dns-accounts/components/dns-accounts-table-row-actions.tsx'
import type { ColumnDef } from '@tanstack/react-table'
import { useAuthStore } from '@/stores/authStore.ts'

function bytesToGb(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

const statusColor: Record<DNSAccountStatus, 'green' | 'yellow' | 'red'> = {
  active: 'green',
  suspended: 'yellow',
  expired: 'red',
}

export const dnsAccountsColumns: ColumnDef<DNSAccount>[] = [
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
      const [dialogOpen, setDialogOpen] = useState(false)
      const resetUsageMutation = useResetDNSAccountUsageMutation()
      const account = row.original
      // Admin-only action as of this fix: the backend now 403s a reseller
      // calling reset-usage (mirrors the identical WireGuard peer-columns
      // fix) -- hide the button for a reseller session.
      const role = useAuthStore((state) => state.auth.admin?.role)
      const isReseller = role === 'reseller'

      const usageDisplay = (
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
      )

      if (isReseller) {
        return <div className='min-w-[140px] text-nowrap'>{usageDisplay}</div>
      }

      const handleResetUsage = async () => {
        resetUsageMutation.mutateAsync(account.id, {
          onSuccess: () => {
            setDialogOpen(false)
            toast.success('مصرف حساب با موفقیت بازنشانی شد', {
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
          title='بازنشانی مصرف حساب؟'
          description='این کار آمار مصرف حساب را بازنشانی می‌کند. آیا مطمئن هستید؟'
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
    accessorKey: 'current_ip',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='IP فعلی' />
    ),
    cell: ({ row }) => {
      const account = row.original
      if (!account.current_ip) {
        return <span className='text-muted-foreground text-xs'>ثبت‌نشده</span>
      }
      return (
        <div className='flex items-center gap-2'>
          <span className='font-mono text-xs'>{account.current_ip}</span>
          {/* is_online is a coarse "has a registered IP and is active"
              proxy, NOT a live connection signal -- honest neutral label,
              muted badge, no green "online now" pulse. */}
          <ColoredBadge color='gray' text='IP ثبت‌شده' />
        </div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
    enableSorting: false,
  },
  {
    accessorKey: 'duration_days',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='مدت زمان' />
    ),
    cell: ({ row }) => {
      const days = row.getValue('duration_days') as number
      return <div>{days > 0 ? `${days} روز` : 'نامحدود'}</div>
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
    cell: DNSAccountsTableRowActions,
  },
]
