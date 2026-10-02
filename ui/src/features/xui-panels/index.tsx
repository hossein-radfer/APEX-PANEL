import { useXuiPanelsListQuery } from '@/hooks/xui-panel/useXuiPanelsListQuery.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { xuiPanelsColumns } from '@/features/xui-panels/components/xui-panels-columns.tsx'
import { XuiPanelsDialogs } from '@/features/xui-panels/components/xui-panels-dialogs.tsx'
import { XuiPanelsPrimaryButtons } from '@/features/xui-panels/components/xui-panels-primary-buttons.tsx'
import XuiPanelsProvider from '@/features/xui-panels/context/xui-panels-context.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'

export default function XuiPanels() {
  const {
    data: panelsList,
    isLoading: isPanelsListLoading,
    refetch: refetchPanelsList,
    isRefetching: isPanelsListRefetching,
  } = useXuiPanelsListQuery()

  return (
    <XuiPanelsProvider>
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
              پنل‌های X-UI
            </h2>
            <p className='text-muted-foreground'>
              پنل‌های X-UI میزبانی‌شده در خارج را که بسته‌های V2Ray روی آن‌ها
              راه‌اندازی می‌شوند ثبت کنید.
            </p>
          </div>
          <XuiPanelsPrimaryButtons
            refetchPanelsList={refetchPanelsList}
            isPanelsListRefetching={isPanelsListRefetching}
          />
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isPanelsListLoading ? (
            <DataTableSkeleton columns={6} />
          ) : (
            <DataTable data={panelsList ?? []} columns={xuiPanelsColumns} />
          )}
        </div>
      </Main>

      <XuiPanelsDialogs />
    </XuiPanelsProvider>
  )
}
