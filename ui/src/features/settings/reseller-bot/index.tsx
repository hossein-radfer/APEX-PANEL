import ContentSection from '../components/content-section'
import { FaoximaForm } from './faoxima-form'

export default function SettingsResellerBot() {
  return (
    <ContentSection
      title='ربات فروش ایکس'
      desc='با وارد کردن توکن ربات تلگرام خودتان، یک نسخه‌ی کامل و مستقل از ربات فروش ایکس (با همان امکانات کامل فروش V2Ray، وایرگارد و User Manager) مخصوص کسب‌وکار شما راه‌اندازی می‌شود — بدون نیاز به دریافت سورس یا هاست جداگانه.'
    >
      <FaoximaForm />
    </ContentSection>
  )
}
