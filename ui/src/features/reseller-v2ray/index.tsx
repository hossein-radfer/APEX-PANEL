import { useMemo, useState } from 'react'
import { IconPackage, IconRefresh } from '@tabler/icons-react'
import { Loader2Icon } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useV2RayPackagesByResellerQuery } from '@/hooks/v2ray/useV2RayPackagesByResellerQuery.ts'
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
import { resellerV2RayColumns } from '@/features/reseller-v2ray/components/reseller-v2ray-columns.tsx'
import { ResellerV2RayDialogs } from '@/features/reseller-v2ray/components/reseller-v2ray-dialogs.tsx'
import ResellerV2RayProvider, {
  useResellerV2Ray,
} from '@/features/reseller-v2ray/context/reseller-v2ray-context.tsx'

function ResellerV2RayPrimaryButtons({
  resellerId,
  refetch,
  isRefetching,
}: {
  resellerId: number
  refetch: () => void
  isRefetching: boolean
}) {
  const { setOpen } = useResellerV2Ray()

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
        <span className='text-sm font-medium'>بازخوانی</span>
      </Button>

      <Button
        onClick={() => setOpen('add')}
        className='gap-2 transition-all'
        disabled={!resellerId}
      >
        <span className='text-sm font-medium'>افزودن پکیج</span>
        <IconPackage className='h-4 w-4' />
      </Button>
    </div>
  )
}

function ResellerV2RayContent() {
  const [pickedResellerID, setPickedResellerID] = useState('')
  const { data: resellers = [] } = useResellersListQuery()
  const { setOpen, setCurrentRow } = useResellerV2Ray()

  const resellerId = useMemo(() => {
    const parsed = Number.parseInt(pickedResellerID, 10)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null
  }, [pickedResellerID])

  const handleResellerChange = (value: string) => {
    setPickedResellerID(value)
    setOpen(null)
    setCurrentRow(null)
  }

  const {
    data: packagesList,
    isLoading: isPackagesListLoading,
    refetch: refetchPackagesList,
    isRefetching: isPackagesListRefetching,
  } = useV2RayPackagesByResellerQuery(resellerId ?? undefined)

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
              V2Ray نمایندگان
            </h2>
            <p className='text-muted-foreground'>
              یک نماینده را انتخاب کنید تا پکیج‌های V2Ray او را مشاهده و مدیریت کنید
            </p>
          </div>
          <ResellerV2RayPrimaryButtons
            resellerId={resellerId ?? 0}
            refetch={refetchPackagesList}
            isRefetching={isPackagesListRefetching}
          />
        </div>

        <Card className='mb-4'>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>نماینده</CardTitle>
          </CardHeader>
          <CardContent className='max-w-sm space-y-2'>
            <Label htmlFor='reseller-v2ray-select'>یک نماینده را انتخاب کنید</Label>
            <Select
              value={pickedResellerID}
              onValueChange={handleResellerChange}
            >
              <SelectTrigger id='reseller-v2ray-select' className='w-full'>
                <SelectValue placeholder='یک نماینده را انتخاب کنید' />
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
                      {!reseller.canResellV2Ray ? ' — ایجاد غیرفعال است' : ''}
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
                  برای مشاهده پکیج‌ها، یکی از نمایندگان بالا را انتخاب کنید.
                </p>
              </CardContent>
            </Card>
          ) : isPackagesListLoading ? (
            <DataTableSkeleton columns={6} />
          ) : (
            <DataTable
              data={packagesList ?? []}
              columns={resellerV2RayColumns}
            />
          )}
        </div>
      </Main>

      {resellerId && <ResellerV2RayDialogs resellerId={resellerId} />}
    </>
  )
}

export default function ResellerV2Ray() {
  return (
    <ResellerV2RayProvider>
      <ResellerV2RayContent />
    </ResellerV2RayProvider>
  )
}
