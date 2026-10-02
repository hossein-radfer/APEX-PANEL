import {
  UserManagerAccount,
  UserManagerAccountStatus,
} from '@/schema/user-manager.ts'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useUpdateUserManagerAccountStatusMutation } from '@/hooks/user-manager/useUpdateUserManagerAccountStatusMutation.ts'
import { Badge } from '@/components/ui/badge.tsx'
import { Switch } from '@/components/ui/switch.tsx'
import LongText from '@/components/long-text'
import { ResellerAccountTableRowActions } from '@/features/reseller-user-manager/components/reseller-account-table-row-actions.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import type { ColumnDef } from '@tanstack/react-table'

// Mirrors accountColumns exactly, except the row-actions cell uses
// ResellerAccountTableRowActions (wired to the admin "Reseller User
// Manager" page context) instead of the normal AccountTableRowActions. The
// status toggle reuses the plain (non-reseller-scoped) mutation since the
// backend already allows an admin unrestricted access to any account
// regardless of its owning reseller -- same convention as resellerPeersColumns.
export const resellerAccountColumns: ColumnDef<UserManagerAccount>[] = [
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
    cell: ({ row }) => (
      <div className='flex w-fit items-center gap-2 text-nowrap'>
        {row.original.username}
      </div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'protocol',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پروتکل' />
    ),
    cell: ({ row }) => (
      <Badge variant='outline' className='uppercase'>
        {row.getValue('protocol')}
      </Badge>
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
      const account = row.original
      return (
        <div className='min-w-[140px] text-nowrap'>
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
        </div>
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
                  text={s}
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
    cell: ResellerAccountTableRowActions,
  },
]
