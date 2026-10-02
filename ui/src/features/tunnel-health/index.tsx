import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
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

          <TabsContent value='statuses'>
            <StatusesTab />
          </TabsContent>
          <TabsContent value='events'>
            <EventsTab />
          </TabsContent>
          <TabsContent value='actions'>
            <ActionsTab />
          </TabsContent>
          <TabsContent value='analysis'>
            <AnalysisTab />
          </TabsContent>
          <TabsContent value='graph'>
            <GraphTab />
          </TabsContent>
          <TabsContent value='user-manager'>
            <UserManagerProtocolsTab />
          </TabsContent>
          <TabsContent value='policy'>
            <PolicyTab />
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
