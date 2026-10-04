import { useState } from 'react'
import { ColumnDef } from '@tanstack/react-table'
import { IconRestore } from '@tabler/icons-react'
import { Peer, PeerStatus } from '@/schema/peers.ts'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useResetUsageMutation } from '@/hooks/peers/useResetUsageMutation.ts'
import { useUpdatePeerStatusMutation } from '@/hooks/peers/useUpdatePeerStatusMutation.ts'
import { getApiErrorMessage } from '@/lib/api-error.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button.tsx'
import { Switch } from '@/components/ui/switch.tsx'
import { Tooltip, TooltipContent, TooltipTrigger, } from '@/components/ui/tooltip'
import LongText from '@/components/long-text'
import { OfflineBadge, OnlineBadge, } from '@/features/peers/components/activity-badge.tsx'
import { PeersTableRowActions } from '@/features/peers/components/peers-table-row-actions.tsx'
import { ColoredBadge } from '@/features/shared-components/status-badge.tsx'
import { DataTableColumnHeader } from '@/features/shared-components/table/data-table-column-header.tsx'
import { SimpleDialog } from '@/features/shared-components/table/dialogs/simple-dialog.tsx'
import { useAuthStore } from '@/stores/authStore.ts'

const peerStatusLabelsFa: Record<PeerStatus, string> = {
  active: 'فعال',
  inactive: 'غیرفعال',
  expired: 'منقضی‌شده',
  suspended: 'معلق',
}

export const peersColumns: ColumnDef<Peer>[] = [
  {
    id: 'is_active',
    cell: ({ row }) => {
      const peer = row.original
      const updatePeerStatusMutation = useUpdatePeerStatusMutation()

      const handleToggle = () => {
        updatePeerStatusMutation.mutate(peer.id, {
          onSuccess: () => {
            toast.success(
              `وایرگارد با موفقیت ${!peer.disabled ? 'غیرفعال' : 'فعال'} شد`,
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
            id={`status-${peer.id}`}
            checked={!peer.disabled}
            onCheckedChange={handleToggle}
            disabled={updatePeerStatusMutation.isPending}
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
    cell: ({ row }) => {
      const { name, is_online } = row.original
      return (
        <div className='flex w-fit items-center justify-center gap-3 text-nowrap'>
          {is_online ? (
            <OnlineBadge peerName={name} />
          ) : (
            <OfflineBadge peerName={name} />
          )}
          {name}
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
    accessorKey: 'interface',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='اینترفیس' />
    ),
    cell: ({ row }) => (
      <div className='w-fit text-nowrap'>{row.getValue('interface')}</div>
    ),
    meta: {
      className: cn('border-l border-r'),
    },
  },
  {
    accessorKey: 'allowed_address',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='آدرس IP' />
    ),
    cell: ({ row }) => <div>{row.getValue('allowed_address')}</div>,
    sortingFn: (rowA, rowB, columnId) => {
      const ipA = rowA.getValue(columnId) as string
      const ipB = rowB.getValue(columnId) as string

      const getIpParts = (ip: string) => {
        const ipOnly = ip.split('/')[0]
        return ipOnly.split('.').map((num) => parseInt(num, 10))
      }

      const partsA = getIpParts(ipA)
      const partsB = getIpParts(ipB)

      for (let i = 0; i < 4; i++) {
        if (partsA[i] !== partsB[i]) {
          return partsA[i] - partsB[i]
        }
      }

      return 0
    },
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

      const resetUsageMutation = useResetUsageMutation()
      // Admin-only action as of this fix: the backend now 403s a reseller
      // calling reset-usage (the admin's own explicit "a reseller must
      // never be able to zero their own usage" requirement) -- hiding the
      // button for a reseller session avoids showing a control that would
      // otherwise always fail.
      const role = useAuthStore((state) => state.auth.admin?.role)
      const isReseller = role === 'reseller'

      const peer = row.original

      const usageDisplay = (
        <div className='flex flex-col leading-tight'>
          <span className='text-foreground text-sm font-medium'>
            {peer.total_usage} GB
            {peer.traffic_limit && (
              <span className='text-muted-foreground text-sm'>
                {' / '}
                {peer.traffic_limit} GB
              </span>
            )}
          </span>
          {!peer.traffic_limit && (
            <span className='text-muted-foreground text-xs'>نامحدود</span>
          )}
        </div>
      )

      if (isReseller) {
        return <div className='min-w-[140px] text-nowrap'>{usageDisplay}</div>
      }

      const handleResetUsage = async () => {
        resetUsageMutation.mutateAsync(peer.id, {
          onSuccess: () => {
            setDialogOpen(false)
            toast.success('مصرف وایرگارد با موفقیت بازنشانی شد', {
              duration: 5000,
            })
          },
          onError: (error) => {
            setDialogOpen(false)
            toast.error(
              getApiErrorMessage(error, 'بازنشانی مصرف وایرگارد ناموفق بود.')
            )
          },
        })
      }

      return (
        <SimpleDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          title='بازنشانی مصرف وایرگارد؟'
          description='این کار آمار مصرف وایرگارد را بازنشانی می‌کند. مطمئن هستید؟'
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
    accessorKey: 'bandwidth',
    header: ({ column }) => (
      <DataTableColumnHeader column={column} title='پهنای باند' />
    ),
    cell: ({ row }) => {
      const { download_bandwidth, upload_bandwidth } = row.original
      return (
        <div className='w-fit min-w-[150px] text-nowrap'>
          {row.original ? (
            <div className='text-foreground flex items-center gap-2 text-sm'>
              <div className='flex items-center gap-1'>
                <span className='text-muted-foreground'>↓</span>
                <span>{download_bandwidth || 'نامحدود'}</span>
              </div>
              <span className='text-muted-foreground'>/</span>
              <div className='flex items-center gap-1'>
                <span className='text-muted-foreground'>↑</span>
                <span>{upload_bandwidth || 'نامحدود'}</span>
              </div>
            </div>
          ) : (
            <Badge
              variant='outline'
              className='text-muted-foreground rounded-sm text-xs'
            >
              نامحدود
            </Badge>
          )}
        </div>
      )
    },
    meta: {
      className: cn('border-l border-r'),
    },
    enableSorting: true,
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
            ? status.map((status: PeerStatus, idx: number) => (
                <ColoredBadge
                  key={idx}
                  color={
                    status === 'active'
                      ? 'green'
                      : status === 'inactive'
                        ? 'gray'
                        : status === 'expired'
                          ? 'yellow'
                          : 'red'
                  }
                  text={peerStatusLabelsFa[status] ?? status}
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
    cell: PeersTableRowActions,
  },
]
