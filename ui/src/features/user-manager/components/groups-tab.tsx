import { useUserManagerGroupsQuery } from '@/hooks/user-manager/useUserManagerGroupsQuery.ts'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table.tsx'

interface Props {
  // When set (a self-serve reseller), only groups in this list are shown --
  // matches the backend's "must be explicitly assigned" enforcement exactly
  // (see ensureResellerCanUseGroup), so a reseller never sees a group here
  // they aren't actually allowed to use when creating an account.
  allowedGroupNames?: string[]
}

export function GroupsTab({ allowedGroupNames }: Props) {
  const { data: allGroups = [], isLoading } = useUserManagerGroupsQuery()

  const groups =
    allowedGroupNames !== undefined
      ? allGroups.filter((g) => allowedGroupNames.includes(g.name))
      : allGroups

  return (
    <div className='space-y-2'>
      <p className='text-muted-foreground text-sm'>
        گروه‌های کاربری تعریف‌شده در User Manager روتر. گروه‌ها را در Winbox
        بسازید یا ویرایش کنید -- این پنل فقط آن‌ها را نمایش می‌دهد.
      </p>

      {isLoading ? (
        <div className='space-y-2'>
          <Skeleton className='h-8 w-full' />
          <Skeleton className='h-8 w-full' />
          <Skeleton className='h-8 w-full' />
        </div>
      ) : groups.length === 0 ? (
        <EmptyState
          message={
            allowedGroupNames !== undefined
              ? 'هنوز هیچ گروهی به شما اختصاص داده نشده است. با مدیر خود تماس بگیرید.'
              : 'هیچ گروهی روی روتر یافت نشد.'
          }
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>نام</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {groups.map((group) => (
              <TableRow key={group.name}>
                <TableCell>{group.name}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
