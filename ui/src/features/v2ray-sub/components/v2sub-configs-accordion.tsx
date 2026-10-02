import { useState } from 'react'
import { V2RayPackageShareDetails } from '@/schema/v2ray.ts'
import { BYTES_PER_GB } from '@/schema/reseller.ts'
import { ChevronDownIcon, LinkIcon, QrCodeIcon } from 'lucide-react'
import { countryFlagFor } from '@/features/v2ray-sub/lib/country-flag.ts'
import { V2SubCopyButton } from '@/features/v2ray-sub/components/v2sub-copy-button.tsx'

interface Props {
  details: V2RayPackageShareDetails
  onShowQr: (value: string) => void
  onCopied: (message: string) => void
}

function shorten(link: string, max = 42): string {
  if (link.length <= max) return link
  return `${link.slice(0, max)}...`
}

export function V2SubConfigsAccordion({ details, onShowQr, onCopied }: Props) {
  const [open, setOpen] = useState(true)

  const totalConfigCount = details.locations.filter((l) => l.config_link).length

  const handleCopyAll = () => {
    const all = details.locations
      .map((l) => l.config_link)
      .filter(Boolean)
      .join('\n')
    return all
  }

  return (
    <div className='v2sub-card'>
      <button
        type='button'
        className='v2sub-accordion-header'
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <span>کانفیگ‌ها</span>
        <ChevronDownIcon
          size={18}
          className={`v2sub-accordion-chevron ${open ? 'v2sub-open' : ''}`}
        />
      </button>

      {open && (
        <div className='v2sub-accordion-body'>
          <div className='v2sub-box'>
            <div className='v2sub-config-row'>
              <div className='v2sub-config-row-main'>
                <div className='v2sub-config-title'>لینک اشتراک</div>
                <div className='v2sub-config-sub'>
                  {shorten(details.subscription_url)}
                </div>
              </div>
              <div className='v2sub-config-actions'>
                <button
                  type='button'
                  className='v2sub-icon-btn'
                  aria-label='نمایش کد QR'
                  title='نمایش کد QR'
                  onClick={() => onShowQr(details.subscription_url)}
                >
                  <QrCodeIcon size={15} />
                </button>
                <V2SubCopyButton
                  getText={() => details.subscription_url}
                  onCopied={() => onCopied('کپی شد')}
                  label='کپی لینک اشتراک'
                />
              </div>
            </div>
          </div>

          <div className='v2sub-box'>
            <div className='v2sub-config-row'>
              <div className='v2sub-config-row-main'>
                <div className='v2sub-config-title'>کپی همه کانفیگ‌ها</div>
                <div className='v2sub-config-sub' style={{ direction: 'rtl', textAlign: 'right' }}>
                  {totalConfigCount} کانفیگ
                </div>
              </div>
              <div className='v2sub-config-actions'>
                <V2SubCopyButton
                  getText={handleCopyAll}
                  onCopied={() => onCopied('همه کانفیگ‌ها کپی شد')}
                  label='کپی همه'
                />
              </div>
            </div>
          </div>

          {details.locations.map((location, i) => {
            const days = details.days_remaining
            const gb = (location.used_bytes / BYTES_PER_GB).toFixed(1)
            return (
              <div className='v2sub-box' key={`${location.title}-${i}`}>
                <div className='v2sub-config-row'>
                  <div className='v2sub-config-row-main'>
                    <div className='v2sub-config-title'>
                      {countryFlagFor(location.title)} {location.title}
                      {'  '}📊{gb}GB
                      {days != null ? `  ⏳${days} روز` : ''}
                    </div>
                    <div className='v2sub-config-sub'>
                      <LinkIcon
                        size={11}
                        style={{ display: 'inline', marginLeft: 4, verticalAlign: '-1px' }}
                      />
                      {location.protocol ? `${location.protocol} · ` : ''}
                      {shorten(location.config_link, 30)}
                    </div>
                  </div>
                  <div className='v2sub-config-actions'>
                    <button
                      type='button'
                      className='v2sub-icon-btn'
                      aria-label='نمایش کد QR'
                      title='نمایش کد QR'
                      disabled={!location.config_link}
                      onClick={() =>
                        location.config_link && onShowQr(location.config_link)
                      }
                    >
                      <QrCodeIcon size={15} />
                    </button>
                    <V2SubCopyButton
                      getText={() => location.config_link}
                      onCopied={() => onCopied('کپی شد')}
                      label='کپی لینک'
                    />
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
