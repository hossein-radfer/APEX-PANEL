import { useDNSPanelsListQuery } from '@/hooks/dns-panel/useDNSPanelsListQuery.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { dnsPanelsColumns } from '@/features/dns-panels/components/dns-panels-columns.tsx'
import { DNSPanelsDialogs } from '@/features/dns-panels/components/dns-panels-dialogs.tsx'
import { DNSPanelsPrimaryButtons } from '@/features/dns-panels/components/dns-panels-primary-buttons.tsx'
import DNSPanelsProvider from '@/features/dns-panels/context/dns-panels-context.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'

export default function DNSPanels() {
  const {
    data: panelsList,
    isLoading: isPanelsListLoading,
    refetch: refetchPanelsList,
    isRefetching: isPanelsListRefetching,
  } = useDNSPanelsListQuery()

  return (
    <DNSPanelsProvider>
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
              پنل‌های DNS
            </h2>
            <p className='text-muted-foreground'>
              پنل‌های doctor-dns میزبانی‌شده در خارج را که حساب‌های Smart DNS
              روی آن‌ها راه‌اندازی می‌شوند ثبت کنید.
            </p>
          </div>
          <DNSPanelsPrimaryButtons
            refetchPanelsList={refetchPanelsList}
            isPanelsListRefetching={isPanelsListRefetching}
          />
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isPanelsListLoading ? (
            <DataTableSkeleton columns={5} />
          ) : (
            <DataTable data={panelsList ?? []} columns={dnsPanelsColumns} />
          )}
        </div>
      </Main>

      <DNSPanelsDialogs />
    </DNSPanelsProvider>
  )
}
