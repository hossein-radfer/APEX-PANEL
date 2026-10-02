import ContentSection from '../components/content-section'
import { AccountForm } from './account-form'

export default function SettingsAccount() {
  return (
    <ContentSection title='حساب کاربری' desc='به‌روزرسانی اطلاعات ورود حساب کاربری شما.'>
      <AccountForm />
    </ContentSection>
  )
}
