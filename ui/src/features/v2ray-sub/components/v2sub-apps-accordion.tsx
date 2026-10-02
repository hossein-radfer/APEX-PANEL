import { useState } from 'react'
import { ChevronDownIcon, DownloadIcon, PlusCircleIcon } from 'lucide-react'
import { APPS_CATALOG } from '@/features/v2ray-sub/lib/apps-catalog.ts'
import { buildDeepLinkUrl } from '@/features/v2ray-sub/lib/deep-link.ts'
import { DetectedOS } from '@/features/v2ray-sub/lib/detect-os.ts'

interface Props {
  subscriptionUrl: string
  detectedOS: DetectedOS
}

export function V2SubAppsAccordion({ subscriptionUrl, detectedOS }: Props) {
  const [openPlatform, setOpenPlatform] = useState<DetectedOS | null>(
    detectedOS !== 'unknown' ? detectedOS : APPS_CATALOG[0]?.id ?? null
  )

  return (
    <div className='v2sub-card'>
      <div className='v2sub-header-title' style={{ marginBottom: 8 }}>
        اپلیکیشن‌ها
      </div>

      {APPS_CATALOG.map((platform) => {
        const isOpen = openPlatform === platform.id
        const isYourOS = detectedOS === platform.id

        return (
          <div key={platform.id}>
            <button
              type='button'
              className='v2sub-platform-header'
              onClick={() => setOpenPlatform(isOpen ? null : platform.id)}
              aria-expanded={isOpen}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                {platform.label}
                {isYourOS && <span className='v2sub-your-os-badge'>سیستم عامل شما</span>}
              </span>
              <ChevronDownIcon
                size={16}
                className={`v2sub-accordion-chevron ${isOpen ? 'v2sub-open' : ''}`}
              />
            </button>

            {isOpen && (
              <div style={{ paddingBottom: 8 }}>
                {platform.apps.map((app) => {
                  const bareImportUrl = app.buildImportUrl?.(subscriptionUrl) ?? null
                  const isAndroid = detectedOS === 'android'
                  const importUrl = bareImportUrl
                    ? buildDeepLinkUrl(bareImportUrl, app.androidPackageId, app.downloadUrl, isAndroid)
                    : null

                  // A plain <a href="customscheme://..."> click is
                  // unreliable across mobile browsers/in-app WebViews (a
                  // confirmed, reported bug -- clicking it opened the app
                  // but never actually imported the subscription). A
                  // script-driven window.location.href assignment is
                  // consistently more reliable for triggering a custom-
                  // scheme/intent:// navigation, so this is a <button>, not
                  // an <a>, for the import action specifically (the plain
                  // Download link above stays a normal <a>, that one's
                  // fine as-is).
                  const handleAddSubscription = () => {
                    if (!importUrl) return
                    window.location.href = importUrl
                  }

                  return (
                    <div className='v2sub-app-card' key={app.name}>
                      <div className='v2sub-app-icon'>{app.name.slice(0, 1)}</div>
                      <div className='v2sub-app-info'>
                        <div className='v2sub-app-name-row'>
                          <span className='v2sub-app-name'>{app.name}</span>
                          {app.recommended && (
                            <span className='v2sub-recommended-badge'>پیشنهادی</span>
                          )}
                        </div>
                        <div className='v2sub-app-desc'>{app.description}</div>
                      </div>
                      <div className='v2sub-app-actions'>
                        <a
                          className='v2sub-app-btn'
                          href={app.downloadUrl}
                          target='_blank'
                          rel='noreferrer'
                        >
                          <DownloadIcon
                            size={12}
                            style={{ display: 'inline', marginLeft: 4, verticalAlign: '-2px' }}
                          />
                          دانلود
                        </a>
                        {importUrl && (
                          <button
                            type='button'
                            className='v2sub-app-btn v2sub-app-btn-primary'
                            onClick={handleAddSubscription}
                          >
                            <PlusCircleIcon
                              size={12}
                              style={{ display: 'inline', marginLeft: 4, verticalAlign: '-2px' }}
                            />
                            افزودن اشتراک
                          </button>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
