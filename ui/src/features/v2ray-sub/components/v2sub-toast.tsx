import { useEffect } from 'react'
import { CheckIcon } from 'lucide-react'

interface Props {
  message: string | null
  onDismiss: () => void
}

// A small bottom-of-screen toast, per the exact spec: shown after a copy
// action, auto-dismisses after a few seconds. Deliberately not the app's
// shared `sonner` toast -- this page has its own fixed visual system, see
// v2ray-sub.css's own header comment.
export function V2SubToast({ message, onDismiss }: Props) {
  useEffect(() => {
    if (!message) return
    const timer = setTimeout(onDismiss, 2200)
    return () => clearTimeout(timer)
  }, [message, onDismiss])

  if (!message) return null

  return (
    <div className='v2sub-toast'>
      <CheckIcon size={14} />
      {message}
    </div>
  )
}
