import { FileText, Stethoscope } from 'lucide-react'
import { TicketMessage } from '@/schema/help-center.ts'
import { useAttachmentUrl } from '@/hooks/help-center/useAttachmentUrl.ts'
import { cn } from '@/lib/utils'

interface MessageBubbleProps {
  message: TicketMessage
  isOwn: boolean
}

function formatBytes(bytes?: number): string {
  if (!bytes) return ''
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB']
  let value = bytes
  let unitIndex = -1
  do {
    value /= 1024
    unitIndex++
  } while (value >= 1024 && unitIndex < units.length - 1)
  return `${value.toFixed(1)} ${units[unitIndex]}`
}

function AttachmentContent({ message }: { message: TicketMessage }) {
  const url = useAttachmentUrl(message.TicketID, message.ID)

  if (message.Kind === 'voice') {
    return url ? <audio controls src={url} className='h-10 max-w-[240px]' /> : null
  }

  return (
    <a
      href={url ?? undefined}
      download={message.AttachmentName}
      target='_blank'
      rel='noreferrer'
      className={cn(
        'flex items-center gap-2 underline-offset-2 hover:underline',
        !url && 'pointer-events-none opacity-60'
      )}
    >
      {message.Kind === 'diagnostic' ? (
        <Stethoscope className='size-4 shrink-0' />
      ) : (
        <FileText className='size-4 shrink-0' />
      )}
      <span className='min-w-0'>
        <span className='block truncate'>{message.AttachmentName || 'فایل پیوست'}</span>
        {message.AttachmentSizeBytes ? (
          <span className='block text-xs opacity-75'>{formatBytes(message.AttachmentSizeBytes)}</span>
        ) : null}
      </span>
    </a>
  )
}

export function MessageBubble({ message, isOwn }: MessageBubbleProps) {
  return (
    <div className={cn('flex flex-col gap-1', isOwn ? 'items-end' : 'items-start')}>
      <div
        className={cn(
          'max-w-[80%] rounded-2xl px-3.5 py-2.5 text-sm',
          isOwn
            ? 'bg-primary text-primary-foreground rounded-br-sm'
            : 'bg-muted text-foreground rounded-bl-sm'
        )}
      >
        {message.Kind !== 'text' && message.AttachmentPath && <AttachmentContent message={message} />}
        {message.Body && <p className='mt-1 whitespace-pre-wrap first:mt-0'>{message.Body}</p>}
      </div>
      <span className='text-muted-foreground px-1 text-[11px]'>
        {new Date(message.CreatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
      </span>
    </div>
  )
}
