import { useUserManagerProfilesQuery } from '@/hooks/user-manager/useUserManagerProfilesQuery.ts'
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
  // When set (a self-serve reseller), only profiles in this list are shown
  // -- matches the backend's "must be explicitly assigned" enforcement
  // exactly (see ensureResellerCanUseProfile).
  allowedProfileNames?: string[]
}

export function ProfilesTab({ allowedProfileNames }: Props) {
  const { data: allProfiles = [], isLoading } = useUserManagerProfilesQuery()

  const profiles =
    allowedProfileNames !== undefined
      ? allProfiles.filter((p) => allowedProfileNames.includes(p.name))
      : allProfiles

  return (
    <div className='space-y-2'>
      <p className='text-muted-foreground text-sm'>
        پروفایل‌های تعریف‌شده در User Manager روتر. پروفایل‌ها را در Winbox
        بسازید یا ویرایش کنید -- این پنل فقط آن‌ها را نمایش می‌دهد.
      </p>

      {isLoading ? (
        <div className='space-y-2'>
          <Skeleton className='h-8 w-full' />
          <Skeleton className='h-8 w-full' />
          <Skeleton className='h-8 w-full' />
        </div>
      ) : profiles.length === 0 ? (
        <EmptyState
          message={
            allowedProfileNames !== undefined
              ? 'هنوز هیچ پروفایلی به شما اختصاص داده نشده است. با مدیر خود تماس بگیرید.'
              : 'هیچ پروفایلی روی روتر یافت نشد.'
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
            {profiles.map((profile) => (
              <TableRow key={profile.name}>
                <TableCell>{profile.name}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
