import { ColumnDef } from '@tanstack/react-table'
import { IconCoin, IconEdit, IconReceipt, IconTrash, IconWallet } from '@tabler/icons-react'
import { useNavigate } from '@tanstack/react-router'
import { BYTES_PER_GB, Reseller } from '@/schema/reseller.ts'
import { Button } from '@/components/ui/button.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { useResellers } from '@/features/resellers/context/resellers-context.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'

function formatGB(bytes: number): string {
  return (bytes / BYTES_PER_GB).toFixed(2)
}

export const resellersColumns: ColumnDef<Reseller>[] = [
  {
    id: 'isActive',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='فعال' />
    ),
    cell: ({ row }) => (
      <div className='flex items-center justify-center'>
        <ColoredBadge
          color={row.original.isActive ? 'green' : 'red'}
          text={row.original.isActive ? 'فعال' : 'غیرفعال'}
        />
      </div>
    ),
  },
  {
    accessorKey: 'name',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام' />
    ),
    cell: ({ row }) => <div>{row.original.name}</div>,
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'username',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='نام کاربری' />
    ),
    cell: ({ row }) => <div>{row.original.username}</div>,
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'email',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='ایمیل' />
    ),
    cell: ({ row }) => <div>{row.original.email ?? '-'}</div>,
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'usedBytes',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='مصرف' />
    ),
    cell: ({ row }) => {
      const { quotaBytes, usedBytes } = row.original
      const usedGB = formatGB(usedBytes)

      if (quotaBytes === null || quotaBytes === undefined) {
        return <div>{usedGB} گیگابایت / نامحدود</div>
      }

      const quotaGB = formatGB(quotaBytes)
      const percent = quotaBytes > 0 ? Math.min(100, (usedBytes / quotaBytes) * 100) : 0
      const remainingGB = formatGB(Math.max(0, quotaBytes - usedBytes))

      return (
        <div className='space-y-1'>
          <div>
            {usedGB} / {quotaGB} گیگابایت
            <span className='text-muted-foreground ml-1 text-xs'>
              ({remainingGB} گیگابایت باقی‌مانده)
            </span>
          </div>
          <div className='bg-muted h-1.5 w-full max-w-32 overflow-hidden rounded-full'>
            <div
              className={
                percent >= 100
                  ? 'h-full bg-red-500'
                  : percent >= 80
                    ? 'h-full bg-yellow-500'
                    : 'h-full bg-green-500'
              }
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>
      )
    },
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'peerCount',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='کاربران' />
    ),
    cell: ({ row }) => {
      const { peerCount, maxPeers } = row.original
      return (
        <div>
          {peerCount ?? 0}
          {maxPeers != null ? ` / ${maxPeers}` : ' / نامحدود'}
        </div>
      )
    },
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'userManagerUsedBytes',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='مصرف یوزر منیجر' />
    ),
    cell: ({ row }) => {
      const {
        canCreateUserManagerAccounts,
        userManagerQuotaBytes,
        userManagerUsedBytes,
      } = row.original

      if (!canCreateUserManagerAccounts) {
        return <div className='text-muted-foreground text-xs'>غیرفعال</div>
      }

      const usedGB = formatGB(userManagerUsedBytes ?? 0)

      if (userManagerQuotaBytes === null || userManagerQuotaBytes === undefined) {
        return <div>{usedGB} گیگابایت / نامحدود</div>
      }

      const quotaGB = formatGB(userManagerQuotaBytes)
      const used = userManagerUsedBytes ?? 0
      const percent =
        userManagerQuotaBytes > 0
          ? Math.min(100, (used / userManagerQuotaBytes) * 100)
          : 0
      const remainingGB = formatGB(Math.max(0, userManagerQuotaBytes - used))

      return (
        <div className='space-y-1'>
          <div>
            {usedGB} / {quotaGB} گیگابایت
            <span className='text-muted-foreground ml-1 text-xs'>
              ({remainingGB} گیگابایت باقی‌مانده)
            </span>
          </div>
          <div className='bg-muted h-1.5 w-full max-w-32 overflow-hidden rounded-full'>
            <div
              className={
                percent >= 100
                  ? 'h-full bg-red-500'
                  : percent >= 80
                    ? 'h-full bg-yellow-500'
                    : 'h-full bg-green-500'
              }
              style={{ width: `${percent}%` }}
            />
          </div>
        </div>
      )
    },
    meta: { className: 'border-l border-r' },
  },
  {
    accessorKey: 'userManagerAccountCount',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='کاربران یوزر منیجر' />
    ),
    cell: ({ row }) => {
      const {
        canCreateUserManagerAccounts,
        userManagerAccountCount,
        userManagerMaxAccounts,
      } = row.original

      if (!canCreateUserManagerAccounts) {
        return <div className='text-muted-foreground text-xs'>-</div>
      }

      return (
        <div>
          {userManagerAccountCount ?? 0}
          {userManagerMaxAccounts != null ? ` / ${userManagerMaxAccounts}` : ' / نامحدود'}
        </div>
      )
    },
    meta: { className: 'border-l border-r' },
  },
  {
    id: 'actions',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='عملیات' />
    ),
    cell: ({ row }) => {
      const { setOpen, setCurrentRow } = useResellers()
      const navigate = useNavigate()
      return (
        // Confirmed, reported bug: on a narrow (mobile) viewport, these 5
        // touch targets sat directly adjacent with equal gap-2 spacing
        // throughout, so the destructive delete button had no more
        // separation from its neighbors than any other pair here --
        // raising the real risk of tapping delete by mistake while
        // reaching for edit. gap-3 gives every button a bit more breathing
        // room, and the extra ms-2 (margin-start, RTL-aware) plus a
        // vertical divider before the destructive button specifically
        // calls it out as visually distinct from the routine/read actions
        // to its right, mirroring how DropdownMenuSeparator already
        // isolates delete in this codebase's other row-action menus (see
        // peers-table-row-actions.tsx).
        <div className='flex items-center gap-3'>
          <Button
            variant='outline'
            size='sm'
            title='مدیریت کیف پول'
            onClick={() => {
              navigate({ to: '/resellers/$id/wallet', params: { id: String(row.original.id) } })
            }}
          >
            <IconWallet className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            size='sm'
            title='قیمت‌های صورتحساب پرداختی'
            onClick={() => {
              navigate({ to: '/resellers/$id/billing-prices', params: { id: String(row.original.id) } })
            }}
          >
            <IconCoin className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            size='sm'
            title='مدیریت فاکتورها'
            onClick={() => {
              navigate({ to: '/billing/invoices' })
            }}
          >
            <IconReceipt className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            size='sm'
            title='ویرایش نماینده'
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('edit')
            }}
          >
            <IconEdit className='h-4 w-4' />
          </Button>
          <div className='bg-border ms-1 h-6 w-px' aria-hidden='true' />
          <Button
            variant='destructive'
            size='sm'
            title='حذف نماینده'
            className='ms-1'
            onClick={() => {
              setCurrentRow(row.original)
              setOpen('delete')
            }}
          >
            <IconTrash className='h-4 w-4' />
          </Button>
        </div>
      )
    },
  },
]
