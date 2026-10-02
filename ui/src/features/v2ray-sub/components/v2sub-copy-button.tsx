import { useState } from 'react'
import { CheckIcon, ClipboardCopyIcon } from 'lucide-react'
import { copyToClipboard } from '@/lib/clipboard.ts'

interface Props {
  getText: () => string
  onCopied: () => void
  label?: string
}

// Copy icon button with the exact spec'd behavior: on click, copy the
// text, flip to a green checkmark for a moment, and let the caller show
// the bottom toast (V2SubToast) -- shared by every copy action on this
// page (sub link, per-location links, copy-all).
export function V2SubCopyButton({ getText, onCopied, label }: Props) {
  const [copied, setCopied] = useState(false)

  const handleClick = async () => {
    const succeeded = await copyToClipboard(getText())
    if (!succeeded) return
    setCopied(true)
    onCopied()
    setTimeout(() => setCopied(false), 1600)
  }

  return (
    <button
      type='button'
      className={`v2sub-icon-btn ${copied ? 'v2sub-copied' : ''}`}
      onClick={handleClick}
      aria-label={label ?? 'کپی'}
      title={label ?? 'کپی'}
    >
      {copied ? <CheckIcon size={15} /> : <ClipboardCopyIcon size={15} />}
    </button>
  )
}
