import ContentSection from '../components/content-section'
import { AppearanceForm } from './appearance-form'

export default function SettingsAppearance() {
  return (
    <ContentSection
      title='ظاهر'
      desc='ظاهر برنامه را سفارشی کنید. جابجایی خودکار بین تم روز و شب.'
    >
      <AppearanceForm />
    </ContentSection>
  )
}
