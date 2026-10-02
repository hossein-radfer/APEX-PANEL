import { FormEvent, useState } from 'react'
import { KeyRoundIcon } from 'lucide-react'

interface Props {
  onSubmit: (shareId: string) => void
}

// Manual entry form -- shown when the page is opened with no shareId in
// the URL, per the exact spec's "login state" requirement.
export function V2SubLoginForm({ onSubmit }: Props) {
  const [value, setValue] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    const trimmed = value.trim()
    const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
    if (!uuidPattern.test(trimmed)) {
      setError('کد اشتراک نامعتبر است -- لطفاً لینک یا کد را از پنل کپی کنید.')
      return
    }
    setError(null)
    onSubmit(trimmed)
  }

  return (
    <div className='v2sub-root'>
      <div className='v2sub-container' style={{ marginTop: '18vh' }}>
        <div className='v2sub-header'>
          <div className='v2sub-header-title'>ورود به اشتراک</div>
          <div className='v2sub-header-sub'>
            کد یا لینک اشتراکی که از پنل دریافت کرده‌اید را وارد کنید.
          </div>
        </div>

        <form className='v2sub-card' onSubmit={handleSubmit}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ position: 'relative' }}>
              <KeyRoundIcon
                size={16}
                style={{
                  position: 'absolute',
                  top: '50%',
                  right: 14,
                  transform: 'translateY(-50%)',
                  color: 'var(--v2sub-text-muted)',
                }}
              />
              <input
                className='v2sub-login-input'
                style={{ paddingRight: 38 }}
                placeholder='کد اشتراک یا لینک را وارد کنید'
                value={value}
                onChange={(e) => setValue(e.target.value)}
                dir='ltr'
                autoFocus
              />
            </div>
            {error && <p className='v2sub-error-text'>{error}</p>}
            <button type='submit' className='v2sub-btn-primary'>
              ورود
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
