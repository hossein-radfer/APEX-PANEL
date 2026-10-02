import {
  UserManagerAccount,
  UserManagerAccountStatus,
} from '@/schema/user-manager.ts'
import { IconRestore } from '@tabler/icons-react'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useResetUserManagerAccountUsageMutation } from '@/hooks/user-manager/useResetUserManagerAccountUsageMutation.ts'
import { useUpdateUserManagerAccountStatusMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountStatusMutation.ts'
import { Badge } from '@/components/ui/badge.tsx'
import { Button } from '@/components/ui/button.tsx'
import { Switch } from '@/components/ui/switch.tsx'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import LongText from '@/components/long-text'
import { AccountTableRowActions } from '@/features/user-manager/components/account-table-row-actions.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { SimpleDialog } from '@/features/shared-components/table/dialogs/simple-dialog.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import {
  OnlineBadge,
  OfflineBadge,
} from '@/features/peers/components/activity-badge.tsx'
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { useAuthStore } from '@/stores/authStore.ts'

const statusLabelsFa: Record<UserManagerAccountStatus, string> = {
  active: 'فعال',
  inactive: 'غیرفعال',
  expired: 'منقضی‌شده',
  suspended: 'سهمیه تمام‌شده',
}

export const accountColumns: ColumnDef<UserManagerAccount>[] = [
  {
    id: 'is_active',
    cell: ({ row }) => {
      const account = row.original
      const updateStatusMutation = useUpdateUserManagerAccountStatusMutation()

      const handleToggle = () => {
        updateStatusMutation.mutate(account.id, {
          onSuccess: () => {
            toast.success(
              `حساب با موفقیت ${!account.disabled ? 'غیرفعال' : 'فعال'} شد`,
              { duration: 5000 }
            )
          },
        })
      }

      return (
        <div className='flex items-center justify-center'>
          <Switch
            id={`status-${account.id}`}
            checked={!account.disabled}
            onCheckedChange={handleToggle}
            disabled={updateStatusMutation.isPending}
          />
        </div>
      )
    },
    enableSorting: false,
  },
  {
    id: 'username',
    accessorKey: 'username',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام کاربری' />
    ),
    cell: ({ row }) => {
      const { username, is_online } = row.original
      return (
        <div className='flex w-fit items-center gap-3 text-nowrap'>
          {is_online ? (
            <OnlineBadge peerName={username} />
          ) : (
            <OfflineBadge peerName={username} />
          )}
          {username}
        </div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'protocols',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پروتکل‌ها' />
    ),
    // Confirmed, reported bug: an account with many protocols (e.g. all
    // five of L2TP/PPTP/SSTP/IKEv2/OpenVPN) had its badges wrap onto
    // several lines, making that one row far taller than every other row
    // and the table look uneven. Capping the visible badges and folding
    // the rest into a single "+N دیگر" badge (full list on hover) keeps
    // every row's height consistent regardless of how many protocols an
    // account has.
    cell: ({ row }) => {
      const protocols = row.original.protocols
      const maxVisible = 3
      const visible = protocols.slice(0, maxVisible)
      const hidden = protocols.slice(maxVisible)
      return (
        <div className='flex flex-nowrap items-center gap-1'>
          {visible.map((protocol) => (
            <Badge key={protocol} variant='outline' className='uppercase'>
              {protocol}
            </Badge>
          ))}
          {hidden.length > 0 && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Badge variant='secondary' className='cursor-default'>
                  +{hidden.length} دیگر
                </Badge>
              </TooltipTrigger>
              <TooltipContent className='uppercase'>
                {hidden.join('، ')}
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      )
    },
    filterFn: (row, columnId, filterValue: string[]) =>
      (row.getValue(columnId) as string[]).some((p) =>
        filterValue.includes(p)
      ),
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
    accessorKey: 'group',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='گروه' />
    ),
    cell: ({ row }) => <div>{row.getValue('group')}</div>,
    filterFn: (row, columnId, filterValue: string[]) =>
      filterValue.includes(row.getValue(columnId)),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'profile',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پروفایل' />
    ),
    cell: ({ row }) => <div>{row.getValue('profile')}</div>,
    filterFn: (row, columnId, filterValue: string[]) =>
      filterValue.includes(row.getValue(columnId)),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'total_usage',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='ترافیک' />
    ),
    cell: ({ row }) => {
      const [dialogOpen, setDialogOpen] = useState(false)
      const resetUsageMutation = useResetUserManagerAccountUsageMutation()
      const account = row.original
      // Admin-only action as of this fix: the backend now 403s a reseller
      // calling reset-usage (mirrors the identical WireGuard peer-columns
      // fix) -- hide the button for a reseller session.
      const role = useAuthStore((state) => state.auth.admin?.role)
      const isReseller = role === 'reseller'

      const usageDisplay = (
        <div className='flex flex-col leading-tight'>
          <span className='text-foreground text-sm font-medium'>
            {account.total_usage} GB
            {account.traffic_limit && (
              <span className='text-muted-foreground text-sm'>
                {' / '}
                {account.traffic_limit} GB
              </span>
            )}
          </span>
          {!account.traffic_limit && (
            <span className='text-muted-foreground text-xs'>نامحدود</span>
          )}
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
    accessorKey: 'expire',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='انقضا' />
    ),
    cell: ({ row }) => {
      const { expire_time } = row.original
      return (
        <div className='w-fit text-nowrap'>
          {expire_time ? expire_time : 'هرگز'}
        </div>
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
      return (
        <div className='flex space-x-2'>
          {Array.isArray(status)
            ? status.map((s: UserManagerAccountStatus, idx: number) => (
                <ColoredBadge
                  key={idx}
                  color={
                    s === 'active'
                      ? 'green'
                      : s === 'inactive'
                        ? 'gray'
                        : s === 'expired'
                          ? 'yellow'
                          : 'red'
                  }
                  text={statusLabelsFa[s] ?? s}
                />
              ))
            : null}
        </div>
      )
    },
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
    cell: AccountTableRowActions,
  },
]
