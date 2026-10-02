import { useMemo, useState } from 'react'
import { IconRefresh, IconUserPlus } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { usePeersByResellerQuery } from '@/hooks/peers/usePeersByResellerQuery.ts'
import { useResellersListQuery } from '@/hooks/resellers/useResellersListQuery.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'
import { resellerPeersColumns } from '@/features/reseller-peers/components/reseller-peers-columns.tsx'
import { ResellerPeersDialogs } from '@/features/reseller-peers/components/reseller-peers-dialogs.tsx'
import ResellerPeersProvider, {
  useResellerPeers,
} from '@/features/reseller-peers/context/reseller-peers-context.tsx'

function ResellerPeersPrimaryButtons({
  resellerId,
  refetch,
  isRefetching,
}: {
  resellerId: number
  refetch: () => void
  isRefetching: boolean
}) {
  const { setOpen } = useResellerPeers()

  return (
    <div className='flex flex-wrap items-center gap-3'>
      <Button
        variant='outline'
        className={cn(
          'shadow-none',
          isRefetching && 'cursor-not-allowed opacity-70'
        )}
        disabled={isRefetching}
        onClick={refetch}
      >
        {isRefetching ? (
          <Loader2Icon className='h-4 w-4 animate-spin' />
        ) : (
          <IconRefresh className='h-4 w-4' />
        )}
        <span className='text-sm font-medium'>تازه‌سازی</span>
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!resellerId}
      >
        <span className='text-sm font-medium'>افزودن وایرگارد</span>
        <IconUserPlus className='h-4 w-4' />
      </Button>
    </div>
  )
}

function ResellerPeersContent() {
  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery()
  const { setOpen, setCurrentRow } = useResellerPeers()

  const resellerId = useMemo(() => {
    const parsed = Number.parseInt(pickedResellerID, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [pickedResellerID])

  // Switching the selected reseller must close any dialog left open for the
  // previous reseller's peer — otherwise an Edit/Delete/Share dialog can
  // stay open while resellerId now points at a different reseller, and the
  // in-flight mutation would be scoped to the wrong reseller.
  const handleResellerChange = (value: string) => {
    setPickedResellerID(value)
    setOpen(null)
    setCurrentRow(null)
  }

  const {
    data: peersList,
    isLoading: isPeersListLoading,
    refetch: refetchPeersList,
    isRefetching: isPeersListRefetching,
  } = usePeersByResellerQuery(resellerId)

  return (
    <>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>
              وایرگارد نمایندگان
            </h2>
            <p className='text-muted-foreground'>
              یک نماینده را برای مشاهده و مدیریت وایرگاردهای آن انتخاب کنید
            </p>
          </div>
          <ResellerPeersPrimaryButtons
            resellerId={resellerId ?? 0}
            refetch={refetchPeersList}
            isRefetching={isPeersListRefetching}
          />
        </div>

        <Card className='mb-4'>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>نماینده</CardTitle>
          </CardHeader>
          <CardContent className='max-w-sm space-y-2'>
            <Label htmlFor='reseller-peers-select'>یک نماینده انتخاب کنید</Label>
            <Select
              value={pickedResellerID}
              onValueChange={handleResellerChange}
            >
              <SelectTrigger id='reseller-peers-select' className='w-full'>
                <SelectValue placeholder='یک نماینده انتخاب کنید' />
              </SelectTrigger>
              <SelectContent>
                {resellers.length === 0 ? (
                  <div className='text-muted-foreground px-2 py-1.5 text-sm'>
                    نماینده‌ای یافت نشد.
                  </div>
                ) : (
                  resellers.map((reseller) => (
                    <SelectItem key={reseller.id} value={String(reseller.id)}>
                      {reseller.name} ({reseller.username})
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </CardContent>
        </Card>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {!resellerId ? (
            <Card>
              <CardContent className='pt-6'>
                <p className='text-muted-foreground'>
                  برای مشاهده‌ی وایرگاردها، یک نماینده را از بالا انتخاب کنید.
                </p>
              </CardContent>
            </Card>
          ) : isPeersListLoading ? (
            <DataTableSkeleton columns={5} />
          ) : (
            <DataTable
              data={peersList ?? []}
              columns={resellerPeersColumns}
              initialSorting={[{ id: 'allowed_address', desc: false }]}
            />
          )}
        </div>
      </Main>

      {resellerId && <ResellerPeersDialogs resellerId={resellerId} />}
    </>
  )
}

export default function ResellerPeers() {
  return (
    <ResellerPeersProvider>
      <ResellerPeersContent />
    </ResellerPeersProvider>
  )
}
