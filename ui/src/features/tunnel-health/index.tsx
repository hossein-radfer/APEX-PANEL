import { useEffect, useState } from 'react'
import { useServersListQuery } from '@/hooks/servers/useServersListQuery.ts'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ThemeSwitch } from '@/components/theme-switch'
import { ActionsTab } from './components/actions-tab'
import { AnalysisTab } from './components/analysis-tab'
import { EventsTab } from './components/events-tab'
import { GraphTab } from './components/graph-tab'
import { PolicyTab } from './components/policy-tab'
import { RedlineTab } from './components/redline-tab'
import { SettingsTab } from './components/settings-tab'
import { StatusesTab } from './components/statuses-tab'
import { UserManagerProtocolsTab } from './components/user-manager-protocols-tab'

export default function TunnelHealth() {
  // Tunnel health is admin-only (same forbidUnlessAdmin gate as the rest
  // of this controller), so -- unlike User Manager's groups/profiles --
  // there's no reseller-scoped variant to special-case here: one shared
  // picker for every tab that needs a server, same pattern as
  // features/user-manager/index.tsx's own selectedServerId.
  const { data: serversList = [] } = useServersListQuery()
  const [selectedServerId, setSelectedServerId] = useState<
    number | undefined
  >(undefined)
  useEffect(() => {
    if (selectedServerId === undefined && serversList.length >= 1) {
      setSelectedServerId(serversList[0].id)
    }
  }, [selectedServerId, serversList])

  const serverPicker = serversList.length > 1 && (
    <Select
      value={selectedServerId ? String(selectedServerId) : undefined}
      onValueChange={(value) => setSelectedServerId(Number(value))}
    >
      <SelectTrigger className='w-full md:w-80'>
        <SelectValue placeholder='یک سرور انتخاب کنید' />
      </SelectTrigger>
      <SelectContent>
        {serversList.map((server) => (
          <SelectItem key={server.id} value={String(server.id)}>
            {server.name} ({server.ip_address})
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

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
        <div className='mb-4 space-y-1'>
          <h1 className='text-2xl font-bold tracking-tight'>
            سلامت تانل‌ها
          </h1>
          <p className='text-muted-foreground'>
            کشف و پایش خودکار تانل‌های زیرساختی (GRE/IPIP/EoIP و
            اینترفیس‌های وایرگاردی که بین سرورها استفاده می‌شوند -- نه
            وایرگاردهای کاربران)، به‌علاوه پایش جداگانه پروتکل‌های
            User Manager و پنل‌های V2Ray. استخراج کامل
            فایروال/NAT/Mangle/Routing Table گراف وابستگی هر تانل را
            خودکار می‌سازد. در حالت آزمایشی، اقدامات درمانی فقط ثبت و
            نمایش داده می‌شوند و هیچ تغییری روی میکروتیک اعمال نمی‌شود.
          </p>
        </div>

        <Tabs defaultValue='statuses' className='space-y-4'>
          <div className='-mx-4 overflow-x-auto px-4 pb-1 sm:mx-0 sm:overflow-visible sm:px-0'>
            <TabsList className='w-max sm:w-fit'>
              <TabsTrigger value='statuses'>وضعیت فعلی</TabsTrigger>
              <TabsTrigger value='events'>تاریخچه رویدادها</TabsTrigger>
              <TabsTrigger value='actions'>تصمیمات و اقدامات</TabsTrigger>
              <TabsTrigger value='analysis'>تحلیل و پیش‌بینی</TabsTrigger>
              <TabsTrigger value='graph'>گراف وابستگی</TabsTrigger>
              <TabsTrigger value='user-manager'>
                پروتکل‌های User Manager
              </TabsTrigger>
              <TabsTrigger value='policy'>سیاست هر تانل</TabsTrigger>
              <TabsTrigger value='redline'>منطقه امن مدیریتی</TabsTrigger>
              <TabsTrigger value='settings'>تنظیمات</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value='statuses' className='space-y-3'>
            {serverPicker}
            <StatusesTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='events' className='space-y-3'>
            {serverPicker}
            <EventsTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='actions' className='space-y-3'>
            {serverPicker}
            <ActionsTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='analysis' className='space-y-3'>
            {serverPicker}
            <AnalysisTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='graph' className='space-y-3'>
            {serverPicker}
            <GraphTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='user-manager'>
            <UserManagerProtocolsTab />
          </TabsContent>
          <TabsContent value='policy' className='space-y-3'>
            {serverPicker}
            <PolicyTab serverId={selectedServerId} />
          </TabsContent>
          <TabsContent value='redline'>
            <RedlineTab />
          </TabsContent>
          <TabsContent value='settings'>
            <SettingsTab />
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}
