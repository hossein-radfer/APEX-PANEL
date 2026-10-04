import { CardErrorBoundary } from '@/components/card-error-boundary.tsx'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { EtherTrafficCard } from '@/features/security/components/ether-traffic-card.tsx'
import { GeoIPUploadCard } from '@/features/security/components/geoip-upload-card.tsx'
import { IdentitiesCard } from '@/features/security/components/identities-card.tsx'
import { RetentionCard } from '@/features/security/components/retention-card.tsx'
import { ThreatsCard } from '@/features/security/components/threats-card.tsx'

export default function SecurityPage() {
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
          <h1 className='text-2xl font-bold tracking-tight'>امنیت</h1>
          <p className='text-muted-foreground'>
            ردیابی آی‌پی کاربران وایرگارد و یوزرمنجیر، ترافیک لحظه‌ای اتریک، و
            پاکسازی داده‌های قدیمی.
          </p>
        </div>

        <div className='space-y-4'>
          <CardErrorBoundary>
            <GeoIPUploadCard />
          </CardErrorBoundary>
          <CardErrorBoundary>
            <ThreatsCard />
          </CardErrorBoundary>
          <CardErrorBoundary>
            <IdentitiesCard />
          </CardErrorBoundary>
          <CardErrorBoundary>
            <EtherTrafficCard />
          </CardErrorBoundary>
          <CardErrorBoundary>
            <RetentionCard />
          </CardErrorBoundary>
        </div>
      </Main>
    </>
  )
}
