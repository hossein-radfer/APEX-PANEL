import { useV2RayPackagesListQuery } from '@/hooks/v2ray/useV2RayPackagesListQuery.ts'
import { useResellerQuery } from '@/hooks/resellers/useResellerQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { v2rayColumns } from '@/features/v2ray-packages/components/v2ray-columns.tsx'
import { V2RayDialogs } from '@/features/v2ray-packages/components/v2ray-dialogs.tsx'
import { V2RayPrimaryButtons } from '@/features/v2ray-packages/components/v2ray-primary-buttons.tsx'
import V2RayProvider from '@/features/v2ray-packages/context/v2ray-context.tsx'
import { DataTable } from '@/features/shared-components/table/data-table.tsx'
import { DataTableSkeleton } from '@/features/shared-components/table/data-table-skeleton.tsx'

export default function V2RayPackages() {
  const admin = useAuthStore((state) => state.auth.admin)
  const isReseller = admin?.role === 'reseller'
  const isAdmin = admin?.role === 'admin'

  const {
    data: packagesList,
    isLoading: isPackagesListLoading,
    refetch: refetchPackagesList,
    isRefetching: isPackagesListRefetching,
  } = useV2RayPackagesListQuery()

  const { data: selfReseller } = useResellerQuery(
    isReseller ? admin?.reseller_id : null
  )

  const canCreate = isAdmin || Boolean(selfReseller?.canResellV2Ray)

  return (
    <V2RayProvider>
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
              بسته‌های V2Ray
            </h2>
            <p className='text-muted-foreground'>
              مدیریت بسته‌های V2Ray ایجادشده روی پنل‌های x-ui ثبت‌شده‌ی شما در
              این بخش -- سیستمی کاملاً جدا از وایرگاردها و حساب‌های
              یوزرمنجیر.
            </p>
          </div>
          <V2RayPrimaryButtons
            refetchPackagesList={refetchPackagesList}
            isPackagesListRefetching={isPackagesListRefetching}
            canCreate={canCreate}
            packagesList={packagesList ?? []}
          />
        </div>

        {isReseller && !canCreate && (
          <p className='text-muted-foreground mb-4 text-sm'>
            شما در حال حاضر اجازه‌ی ایجاد بسته‌ی V2Ray را ندارید. برای درخواست
            دسترسی با ادمین خود تماس بگیرید.
          </p>
        )}

        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          {isPackagesListLoading ? (
            <DataTableSkeleton columns={6} />
          ) : (
            <DataTable data={packagesList ?? []} columns={v2rayColumns} />
          )}
        </div>
      </Main>

      <V2RayDialogs packagesList={packagesList ?? []} />
    </V2RayProvider>
  )
}
