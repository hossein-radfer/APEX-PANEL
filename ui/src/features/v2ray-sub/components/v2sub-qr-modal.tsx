import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { XIcon } from 'lucide-react'

interface Props {
  value: string | null
  onClose: () => void
}

// Centered, backdrop-blurred QR modal per the exact spec -- opened by
// clicking any QR button (sub link, or a per-location config link).
export function V2SubQrModal({ value, onClose }: Props) {
  const [dataUrl, setDataUrl] = useState<string | null>(null)

  useEffect(() => {
    if (!value) {
      setDataUrl(null)
      return
    }
    let cancelled = false
    QRCode.toDataURL(value, { width: 240, margin: 1 })
      .then((url) => {
        if (!cancelled) setDataUrl(url)
      })
      .catch(() => {
        if (!cancelled) setDataUrl(null)
      })
    return () => {
      cancelled = true
    }
  }, [value])

  useEffect(() => {
    if (!value) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [value, onClose])

  if (!value) return null

  return (
    <div
      className='v2sub-modal-backdrop'
      onClick={onClose}
      role='dialog'
      aria-modal='true'
    >
      <div className='v2sub-modal-card' onClick={(e) => e.stopPropagation()}>
        <div className='v2sub-header-title'>کد QR</div>
        <div className='v2sub-modal-qr'>
          {dataUrl ? (
            <img src={dataUrl} alt='QR Code' width={240} height={240} />
          ) : (
            <div style={{ width: 240, height: 240 }} />
          )}
        </div>
        <button
          type='button'
          className='v2sub-app-btn v2sub-app-btn-primary'
          style={{ width: '100%' }}
          onClick={onClose}
        >
          <XIcon size={14} style={{ display: 'inline', marginLeft: 6, verticalAlign: '-2px' }} />
          بستن
        </button>
      </div>
    </div>
  )
}
