import { useEffect, useState } from 'react'
import { fetchAttachmentBlob } from '@/api/help-center.ts'

// Downloads an attachment through the authenticated axios instance and
// exposes it as a blob: object URL for <img>/<audio src>, revoking it on
// unmount to avoid leaking memory across a long chat session with many
// attachments.
export function useAttachmentUrl(ticketId: number, messageId: number): string | null {
  const [url, setUrl] = useState<string | null>(null)

  useEffect(() => {
    let objectUrl: string | null = null
    let cancelled = false

    fetchAttachmentBlob(ticketId, messageId).then((blobUrl) => {
      if (cancelled) {
        URL.revokeObjectURL(blobUrl)
        return
      }
      objectUrl = blobUrl
      setUrl(blobUrl)
    })

    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [ticketId, messageId])

  return url
}
