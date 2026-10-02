import { usePeersListQuery } from '@/hooks/peers/usePeersListQuery.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { peersColumns } from '@/features/peers/components/peers-columns.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '../shared-components/table/data-table-skeleton.tsx'
import { PeersDialogs } from './components/peers-dialogs.tsx'
import { PeersPrimaryButtons } from './components/peers-primary-buttons.tsx'
import PeersProvider from './context/peers-context.tsx'

export default function Peers() {
  const {
    data: peersList,
    isLoading: isPeersListLoading,
    refetch: refetchPeersList,
    isRefetching: isPeersListRefetching,
  } = usePeersListQuery()

  return (
    <PeersProvider>
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
            <h2 className='text-2xl font-bold tracking-tight'>وایرگارد</h2>
            <p className='text-muted-foreground'>
              مدیریت کانفیگ‌های WireGuard در این بخش
            </p>
          </div>
          <PeersPrimaryButtons
            refetchPeersList={refetchPeersList}
            isPeersListRefetching={isPeersListRefetching}
            peersList={peersList ?? []}
          />
        </div>

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isPeersListLoading ? (
            <DataTableSkeleton columns={5} />
          ) : (
            <DataTable
              data={peersList ?? []}
              columns={peersColumns}
              initialSorting={[{ id: 'allowed_address', desc: false }]}
            />
          )}
        </div>
      </Main>

      <PeersDialogs peersList={peersList ?? []} />
    </PeersProvider>
  )
}
