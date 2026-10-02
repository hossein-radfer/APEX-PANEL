import { useMemo, useState, useEffect } from 'react'
import { IconActivity, IconShare, IconUserCheck, IconUserOff, IconUsers, IconWallet, IconReceipt, IconShieldLock, IconServer2, IconHourglassLow, IconApps } from '@tabler/icons-react'
import { useApplicationsListQuery } from '@/hooks/applications/useApplicationsListQuery.ts'
import { useDeviceDataQuery } from '@/hooks/dashboard/useDeviceDataQuery.ts'
import { usePeersListQuery } from '@/hooks/peers/usePeersListQuery.ts'
import { useResellerActivitySummaryQuery } from '@/hooks/peers/useResellerActivitySummaryQuery.ts'
import { useSelfActivityQuery } from '@/hooks/peers/useSelfActivityQuery.ts'
import { useResellerQuery } from '@/hooks/resellers/useResellerQuery.ts'
import { useResellerQuotaPredictionQuery } from '@/hooks/reports/useResellerQuotaPredictionQuery.ts'
import { useUserManagerSelfSummaryQuery } from '@/hooks/user-manager/useUserManagerSelfSummaryQuery.ts'
import { useV2RayAdminSummaryQuery } from '@/hooks/v2ray/useV2RayAdminSummaryQuery.ts'
import { useV2RaySelfSummaryQuery } from '@/hooks/v2ray/useV2RaySelfSummaryQuery.ts'
import { useDNSAdminSummaryQuery } from '@/hooks/dns-account/useDNSAdminSummaryQuery.ts'
import { useDNSSelfSummaryQuery } from '@/hooks/dns-account/useDNSSelfSummaryQuery.ts'
import { useWalletBalanceQuery, useLedgerHistoryQuery } from '@/hooks/wallet/useWalletQueries.ts'
import { ResellerOnboardingWizard } from '@/features/dashboard/components/reseller-onboarding-wizard.tsx'
import { ConfettiBurst } from '@/components/ui/confetti-burst.tsx'
import { formatCurrencyFa, formatBytesFa, formatNumberFa, protocolLabelFa } from '@/features/reports/lib/format.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { PanelHealthTable } from '@/features/dashboard/components/panel-health-table.tsx'
import { StatsCard } from '@/features/dashboard/components/stats-card.tsx'
import { HighlightStatsCard } from '@/features/dashboard/components/highlight-stats-card.tsx'
import { DashboardSummary } from '@/features/dashboard/components/dashboard-summary.tsx'
import DeviceInfo from '@/features/dashboard/components/device-info.tsx'
import DeviceResource from '@/features/dashboard/components/device-resource.tsx'
import RecentlyOnlineUsers from '@/features/dashboard/components/online-users.tsx'
import { PeersChart } from '@/features/dashboard/components/peers-chart.tsx'
import OnlineUsersSkeleton from '@/features/dashboard/components/skeletons/online-users.skeleton.tsx'
import DeviceStatsSkeleton from '@/features/dashboard/components/skeletons/statistics.skeleton.tsx'
import { TrafficChart } from '@/features/dashboard/components/traffic-chart.tsx'

export default function Dashboard() {
  const role = useAuthStore((state) => state.auth.admin?.role)
  const resellerID = useAuthStore((state) => state.auth.admin?.reseller_id)
  const isReseller = role === 'reseller'
  const { data: deviceData, isLoading: isDeviceDataLoading } =
    useDeviceDataQuery(!isReseller)
  const { data: peersList, isLoading: isPeersListLoading } = usePeersListQuery()
  const { data: walletData, isLoading: isWalletLoading } = useWalletBalanceQuery(
    isReseller ? resellerID : null
  )
  const { data: ledgerEntries, isLoading: isLedgerLoading } = useLedgerHistoryQuery(
    isReseller ? resellerID : null,
    10
  )
  const { data: selfActivity, isLoading: isSelfActivityLoading } =
    useSelfActivityQuery(isReseller)
  const { data: resellerActivitySummary, isLoading: isResellerActivityLoading } =
    useResellerActivitySummaryQuery(1, 20, !isReseller)
  const { data: umSummary, isLoading: isUmSummaryLoading } =
    useUserManagerSelfSummaryQuery(isReseller)
  const { data: v2raySummary, isLoading: isV2raySummaryLoading } =
    useV2RaySelfSummaryQuery(isReseller)
  const { data: v2rayAdminSummary, isLoading: isV2rayAdminSummaryLoading } =
    useV2RayAdminSummaryQuery(!isReseller)
  const { data: dnsSummary, isLoading: isDnsSummaryLoading } =
    useDNSSelfSummaryQuery(isReseller)
  const { data: dnsAdminSummary, isLoading: isDnsAdminSummaryLoading } =
    useDNSAdminSummaryQuery(!isReseller)
  const { data: quotaPrediction, isLoading: isQuotaPredictionLoading } =
    useResellerQuotaPredictionQuery(isReseller)
  const { data: selfReseller } = useResellerQuery(
    isReseller ? resellerID : null
  )
  const { data: applicationsData, isLoading: isApplicationsLoading } =
    useApplicationsListQuery()

  const applicationsSummary = useMemo(() => {
    const apps = applicationsData ?? []
    const activeApps = apps.filter(
      (app) => app.status === 'active' && !app.disabled
    ).length
    const usedBytes = apps.reduce((sum, app) => sum + app.used_bytes, 0)
    return {
      totalApps: apps.length,
      activeApps,
      usedBytes,
    }
  }, [applicationsData])

  const [showOnboarding, setShowOnboarding] = useState(false)
  const [showConfetti, setShowConfetti] = useState(false)
  useEffect(() => {
    if (isReseller && selfReseller && !selfReseller.hasCompletedOnboarding) {
      setShowOnboarding(true)
      setShowConfetti(true)
    }
  }, [isReseller, selfReseller])

  const resellerSummary = useMemo(() => {
    const peers = peersList ?? []
    const activePeers = peers.filter((peer) => !peer.disabled).length
    const disabledPeers = peers.filter((peer) => peer.disabled).length
    const sharedPeers = peers.filter((peer) => peer.is_shared).length
    const totalUsage = peers.reduce(
      (sum, peer) => sum + Number.parseFloat(peer.total_usage || '0'),
      0
    )

    return {
      totalPeers: peers.length,
      activePeers,
      disabledPeers,
      sharedPeers,
      totalUsage: totalUsage.toFixed(2),
      recentPeers: peers.slice(0, 5),
    }
  }, [peersList])

  if (isReseller) {
    return (
      <>
        {resellerID != null && (
          <ResellerOnboardingWizard
            resellerId={resellerID}
            open={showOnboarding}
            onOpenChange={setShowOnboarding}
          />
        )}
        {showConfetti && (
          <ConfettiBurst onDone={() => setShowConfetti(false)} />
        )}

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
              <h2 className='text-2xl font-bold tracking-tight'>داشبورد</h2>
              <p className='text-muted-foreground'>وایرگاردهای اختصاصی و مصرف شما</p>
            </div>
          </div>

          <div className='space-y-4'>
            <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6'>
              <StatsCard
                title='کل وایرگاردها'
                icon={<IconUsers />}
                value={resellerSummary.totalPeers}
                isLoading={isPeersListLoading}
              />
              <StatsCard
                title='آنلاین اکنون'
                icon={<IconActivity />}
                value={selfActivity?.online_peers ?? 0}
                isLoading={isSelfActivityLoading}
              />
              <StatsCard
                title='وایرگاردهای فعال'
                icon={<IconUserCheck />}
                value={resellerSummary.activePeers}
                isLoading={isPeersListLoading}
              />
              <StatsCard
                title='وایرگاردهای غیرفعال'
                icon={<IconUserOff />}
                value={resellerSummary.disabledPeers}
                isLoading={isPeersListLoading}
              />
              <StatsCard
                title='وایرگاردهای اشتراکی'
                icon={<IconShare />}
                value={resellerSummary.sharedPeers}
                isLoading={isPeersListLoading}
              />
              <StatsCard
                title='مصرف امروز'
                icon={<IconActivity />}
                value={`${selfActivity?.today_usage_gb ?? '0'} گیگابایت`}
                isLoading={isSelfActivityLoading}
              />
            </div>

            <div>
              <h2 className='mb-2 text-lg font-semibold tracking-tight'>
                User Manager (L2TP / PPTP / SSTP / OpenVPN)
              </h2>
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
                <StatsCard
                  title='کل حساب‌ها'
                  icon={<IconShieldLock />}
                  value={umSummary?.total_accounts ?? 0}
                  isLoading={isUmSummaryLoading}
                />
                <StatsCard
                  title='آنلاین اکنون'
                  icon={<IconActivity />}
                  value={umSummary?.online_accounts ?? 0}
                  isLoading={isUmSummaryLoading}
                />
                <StatsCard
                  title='مصرف‌شده'
                  icon={<IconActivity />}
                  value={
                    umSummary
                      ? `${(umSummary.used_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : '۰ گیگابایت'
                  }
                  isLoading={isUmSummaryLoading}
                />
                <StatsCard
                  title='باقی‌مانده'
                  icon={<IconActivity />}
                  value={
                    umSummary?.remaining_bytes != null
                      ? `${(umSummary.remaining_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : 'نامحدود'
                  }
                  isLoading={isUmSummaryLoading}
                />
              </div>
            </div>

            <div>
              <h2 className='mb-2 text-lg font-semibold tracking-tight'>
                V2Ray
              </h2>
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
                <StatsCard
                  title='کل پکیج‌ها'
                  icon={<IconServer2 />}
                  value={v2raySummary?.total_packages ?? 0}
                  isLoading={isV2raySummaryLoading}
                />
                <StatsCard
                  title='آنلاین اکنون'
                  icon={<IconActivity />}
                  value={v2raySummary?.online_packages ?? 0}
                  isLoading={isV2raySummaryLoading}
                />
                <StatsCard
                  title='مصرف‌شده'
                  icon={<IconActivity />}
                  value={
                    v2raySummary
                      ? `${(v2raySummary.used_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : '۰ گیگابایت'
                  }
                  isLoading={isV2raySummaryLoading}
                />
                <StatsCard
                  title='باقی‌مانده'
                  icon={<IconActivity />}
                  value={
                    v2raySummary?.remaining_bytes != null
                      ? `${(v2raySummary.remaining_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : 'نامحدود'
                  }
                  isLoading={isV2raySummaryLoading}
                />
              </div>
            </div>

            <div>
              <h2 className='mb-2 text-lg font-semibold tracking-tight'>
                DNS
              </h2>
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
                <StatsCard
                  title='کل حساب‌ها'
                  icon={<IconServer2 />}
                  value={dnsSummary?.total_accounts ?? 0}
                  isLoading={isDnsSummaryLoading}
                />
                <StatsCard
                  title='IP ثبت‌شده'
                  icon={<IconActivity />}
                  value={dnsSummary?.online_accounts ?? 0}
                  isLoading={isDnsSummaryLoading}
                />
                <StatsCard
                  title='مصرف‌شده'
                  icon={<IconActivity />}
                  value={
                    dnsSummary
                      ? `${(dnsSummary.used_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : '۰ گیگابایت'
                  }
                  isLoading={isDnsSummaryLoading}
                />
                <StatsCard
                  title='باقی‌مانده'
                  icon={<IconActivity />}
                  value={
                    dnsSummary?.remaining_bytes != null
                      ? `${(dnsSummary.remaining_bytes / BYTES_PER_GB).toFixed(2)} گیگابایت`
                      : 'نامحدود'
                  }
                  isLoading={isDnsSummaryLoading}
                />
              </div>
            </div>

            <div>
              <h2 className='mb-2 text-lg font-semibold tracking-tight'>
                اپلیکیشن‌ها
              </h2>
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                <StatsCard
                  title='کل اپلیکیشن‌ها'
                  icon={<IconApps />}
                  value={applicationsSummary.totalApps}
                  isLoading={isApplicationsLoading}
                />
                <StatsCard
                  title='فعال'
                  icon={<IconUserCheck />}
                  value={applicationsSummary.activeApps}
                  isLoading={isApplicationsLoading}
                />
                <StatsCard
                  title='مصرف‌شده'
                  icon={<IconActivity />}
                  value={`${(applicationsSummary.usedBytes / BYTES_PER_GB).toFixed(2)} گیگابایت`}
                  isLoading={isApplicationsLoading}
                />
              </div>
            </div>

            <div>
              <h2 className='mb-2 text-lg font-semibold tracking-tight'>
                پیش‌بینی پایان حجم
              </h2>
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                {isQuotaPredictionLoading ? (
                  <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                ) : (
                  quotaPrediction?.protocols.map((protocol) => (
                    <Card key={protocol.protocol}>
                      <CardHeader className='flex flex-row items-center justify-between pb-2'>
                        <CardTitle className='text-sm font-medium'>
                          {protocolLabelFa(protocol.protocol)}
                        </CardTitle>
                        <IconHourglassLow className='text-muted-foreground h-4 w-4' />
                      </CardHeader>
                      <CardContent>
                        {protocol.days_remaining != null ? (
                          <p className='text-2xl font-bold tabular-nums'>
                            {formatNumberFa(protocol.days_remaining)} روز
                          </p>
                        ) : (
                          <p className='text-2xl font-bold'>نامشخص</p>
                        )}
                        <p className='text-muted-foreground text-xs mt-1'>
                          {protocol.quota_bytes == null
                            ? 'حجم نامحدود'
                            : protocol.avg_daily_usage_bytes === 0
                              ? 'بدون مصرف اخیر برای برآورد'
                              : `میانگین مصرف روزانه: ${formatBytesFa(protocol.avg_daily_usage_bytes)}`}
                        </p>
                      </CardContent>
                    </Card>
                  ))
                )}
              </div>
            </div>

            <div className='grid gap-4 lg:grid-cols-3'>
              <div className='lg:col-span-1 flex flex-col gap-4'>
                <HighlightStatsCard
                  // Explicitly "WireGuard" -- a confirmed, reported bug:
                  // this card only ever sums model.Peer.TotalUsage (see
                  // resellerSummary above, which reduces over `peers`
                  // alone), but it renders directly below the V2Ray
                  // stats section in the page layout, with no protocol
                  // name in its own title -- a reseller with NO V2Ray
                  // permission at all (can_resell_v2_ray=false) saw
                  // their real WireGuard usage here and reasonably read
                  // it as V2Ray usage, since visually it reads as
                  // belonging to the section right above it. The V2Ray
                  // section's own "Used" card (right above, tied to
                  // v2raySummary) already correctly shows 0 GB for such
                  // a reseller; this card was the actual source of the
                  // confusion.
                  title='کل مصرف WireGuard'
                  icon={<IconUsers />}
                  value={resellerSummary.totalUsage}
                  suffix='گیگابایت'
                  isLoading={isPeersListLoading}
                />
                <Card>
                  <CardHeader className='flex flex-row items-center justify-between pb-2'>
                    <CardTitle className='text-sm font-medium'>موجودی کیف پول</CardTitle>
                    <IconWallet className='text-muted-foreground h-4 w-4' />
                  </CardHeader>
                  <CardContent>
                    {isWalletLoading ? (
                      <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                    ) : walletData ? (
                      <div>
                        <p className='text-2xl font-bold tabular-nums'>
                          {formatCurrencyFa(walletData.balance_amount)}
                        </p>
                        <p className='text-muted-foreground text-xs mt-1'>اعتبار موجود</p>
                      </div>
                    ) : (
                      <p className='text-muted-foreground text-sm'>هنوز کیف پولی وجود ندارد</p>
                    )}
                  </CardContent>
                </Card>
              </div>

              <Card className='lg:col-span-1'>
                <CardHeader className='flex flex-row items-center justify-between pb-2'>
                  <CardTitle className='text-sm font-medium'>تراکنش‌های اخیر</CardTitle>
                  <IconReceipt className='text-muted-foreground h-4 w-4' />
                </CardHeader>
                <CardContent className='space-y-2'>
                  {isLedgerLoading ? (
                    <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                  ) : ledgerEntries && ledgerEntries.length > 0 ? (
                    ledgerEntries.map((entry) => (
                      <div key={entry.id} className='flex items-center justify-between text-sm'>
                        <div>
                          <p className='font-medium capitalize'>{entry.entry_type.toLowerCase()}</p>
                          <p className='text-muted-foreground text-xs'>{entry.description || entry.reference_type}</p>
                        </div>
                        <p className={`tabular-nums ${entry.amount >= 0 ? 'text-green-600 font-medium' : 'text-red-600 font-medium'}`}>
                          {entry.amount >= 0 ? '+' : ''}{formatCurrencyFa(entry.amount)}
                        </p>
                      </div>
                    ))
                  ) : (
                    <p className='text-muted-foreground text-sm'>هنوز تراکنشی ثبت نشده است.</p>
                  )}
                </CardContent>
              </Card>

              <Card className='lg:col-span-1'>
                <CardHeader>
                  <CardTitle className='text-sm font-medium'>وایرگاردهای اخیر</CardTitle>
                </CardHeader>
                <CardContent className='space-y-3'>
                  {resellerSummary.recentPeers.length === 0 ? (
                    <p className='text-muted-foreground text-sm'>هنوز وایرگاردی اختصاص داده نشده است.</p>
                  ) : (
                    resellerSummary.recentPeers.map((peer) => (
                      <div
                        key={peer.id}
                        className='flex items-center justify-between rounded-lg border px-4 py-3'
                      >
                        <div>
                          <p className='font-medium'>{peer.name}</p>
                          <p className='text-muted-foreground text-xs'>
                            {peer.allowed_address}
                          </p>
                        </div>
                        <div className='text-right'>
                          <p className='text-sm font-medium'>{peer.total_usage} گیگابایت</p>
                          <p className='text-muted-foreground text-xs'>
                            {peer.disabled ? 'غیرفعال' : 'فعال'}
                          </p>
                        </div>
                      </div>
                    ))
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        </Main>
      </>
    )
  }

  return (
    <>
      {/* ===== Top Heading ===== */}
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      {/* ===== Main ===== */}
      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>داشبورد</h2>
            <p className='text-muted-foreground'>
              خلاصه‌ی سرورها و کاربران
            </p>
          </div>
        </div>
        <Tabs
          orientation='vertical'
          defaultValue='overview'
          className='space-y-4'
        >
          <TabsContent value='overview' className='space-y-4'>
            <DashboardSummary
              deviceData={deviceData}
              isLoading={isDeviceDataLoading}
            />

            <Card>
              <CardHeader className='flex flex-row items-center justify-between pb-2'>
                <CardTitle className='text-sm font-medium'>
                  فعالیت نمایندگان
                </CardTitle>
                <IconActivity className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                {isResellerActivityLoading ? (
                  <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                ) : resellerActivitySummary && resellerActivitySummary.resellers.length > 0 ? (
                  <div className='space-y-3'>
                    <div className='flex flex-wrap gap-6'>
                      <div>
                        <p className='text-muted-foreground text-xs'>مجموع آنلاین (همه‌ی نمایندگان)</p>
                        <p className='text-xl font-bold'>{formatNumberFa(resellerActivitySummary.total_online_peers)}</p>
                      </div>
                      <div>
                        <p className='text-muted-foreground text-xs'>مصرف امروز (همه‌ی نمایندگان)</p>
                        <p className='text-xl font-bold'>{formatNumberFa(Number(resellerActivitySummary.total_today_usage_gb))} گیگابایت</p>
                      </div>
                    </div>
                    <div className='overflow-hidden rounded-lg border'>
                      <Table>
                        <TableHeader>
                          <TableRow className='bg-muted/40'>
                            <TableHead className='text-start'>نماینده</TableHead>
                            <TableHead className='text-start'>آنلاین</TableHead>
                            <TableHead className='text-start'>کل کاربران</TableHead>
                            <TableHead className='text-start'>مصرف امروز</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {resellerActivitySummary.resellers.map((r) => (
                            <TableRow key={r.reseller_id}>
                              <TableCell className='font-medium'>{r.reseller_name}</TableCell>
                              <TableCell>{formatNumberFa(r.online_peers)}</TableCell>
                              <TableCell>{formatNumberFa(r.total_peers)}</TableCell>
                              <TableCell>{formatNumberFa(Number(r.today_usage_gb))} گیگابایت</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </div>
                ) : (
                  <EmptyState message='هنوز هیچ کاربری متعلق به نماینده‌ای وجود ندارد.' />
                )}
              </CardContent>
            </Card>

            <Card interactive>
              <CardHeader className='flex flex-row items-center justify-between pb-2'>
                <CardTitle className='text-sm font-medium'>
                  پنل‌های V2Ray
                </CardTitle>
                <IconServer2 className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                {isV2rayAdminSummaryLoading ? (
                  <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                ) : v2rayAdminSummary && v2rayAdminSummary.panels.length > 0 ? (
                  <div className='space-y-3'>
                    <div className='flex flex-wrap gap-6'>
                      <div>
                        <p className='text-muted-foreground text-xs'>کل پکیج‌ها</p>
                        <p className='text-xl font-bold'>{formatNumberFa(v2rayAdminSummary.total_packages)}</p>
                      </div>
                      <div>
                        <p className='text-muted-foreground text-xs'>لوکیشن‌های آنلاین</p>
                        <p className='text-xl font-bold'>
                          {formatNumberFa(v2rayAdminSummary.online_locations)} / {formatNumberFa(v2rayAdminSummary.total_locations)}
                        </p>
                      </div>
                      <div>
                        <p className='text-muted-foreground text-xs'>کل مصرف</p>
                        <p className='text-xl font-bold'>
                          {formatNumberFa(Number((v2rayAdminSummary.total_used_bytes / BYTES_PER_GB).toFixed(2)))} گیگابایت
                        </p>
                      </div>
                    </div>
                    <PanelHealthTable
                      itemCountLabel='لوکیشن‌ها'
                      rows={v2rayAdminSummary.panels.map((p) => ({
                        panelId: p.panel_id,
                        panelName: p.panel_name,
                        onlineCount: p.online_count,
                        itemCount: p.location_count,
                        hasRecentError: p.has_recent_error,
                      }))}
                    />
                  </div>
                ) : (
                  <EmptyState message='هنوز هیچ پنل V2Ray ثبت نشده است.' />
                )}
              </CardContent>
            </Card>

            <Card interactive>
              <CardHeader className='flex flex-row items-center justify-between pb-2'>
                <CardTitle className='text-sm font-medium'>
                  پنل‌های DNS
                </CardTitle>
                <IconServer2 className='text-muted-foreground h-4 w-4' />
              </CardHeader>
              <CardContent>
                {isDnsAdminSummaryLoading ? (
                  <p className='text-muted-foreground text-sm'>در حال بارگذاری...</p>
                ) : dnsAdminSummary && dnsAdminSummary.panels.length > 0 ? (
                  <div className='space-y-3'>
                    <div className='flex flex-wrap gap-6'>
                      <div>
                        <p className='text-muted-foreground text-xs'>کل حساب‌ها</p>
                        <p className='text-xl font-bold'>{formatNumberFa(dnsAdminSummary.total_accounts)}</p>
                      </div>
                      <div>
                        <p className='text-muted-foreground text-xs'>IP ثبت‌شده</p>
                        <p className='text-xl font-bold'>
                          {formatNumberFa(dnsAdminSummary.online_accounts)} / {formatNumberFa(dnsAdminSummary.total_accounts)}
                        </p>
                      </div>
                      <div>
                        <p className='text-muted-foreground text-xs'>کل مصرف</p>
                        <p className='text-xl font-bold'>
                          {formatNumberFa(Number((dnsAdminSummary.total_used_bytes / BYTES_PER_GB).toFixed(2)))} گیگابایت
                        </p>
                      </div>
                    </div>
                    <PanelHealthTable
                      itemCountLabel='حساب‌ها'
                      rows={dnsAdminSummary.panels.map((p) => ({
                        panelId: p.panel_id,
                        panelName: p.panel_name,
                        onlineCount: p.online_count,
                        itemCount: p.account_count,
                        hasRecentError: p.has_recent_error,
                      }))}
                    />
                  </div>
                ) : (
                  <EmptyState message='هنوز هیچ پنل DNS ثبت نشده است.' />
                )}
              </CardContent>
            </Card>

            <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
              {isDeviceDataLoading ? (
                <DeviceStatsSkeleton type='base' />
              ) : (
                <DeviceInfo stats={deviceData} />
              )}

              {isDeviceDataLoading ? (
                <DeviceStatsSkeleton type='resource' />
              ) : (
                <DeviceResource stats={deviceData?.DeviceInfo} />
              )}

              <PeersChart
                isLoading={isDeviceDataLoading}
                stats={deviceData?.PeerInfo}
              />

              {isDeviceDataLoading ? (
                <OnlineUsersSkeleton />
              ) : (
                <RecentlyOnlineUsers
                  peers={deviceData?.PeerInfo?.recent_online_peers ?? []}
                />
              )}

              <TrafficChart />
            </div>
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}
