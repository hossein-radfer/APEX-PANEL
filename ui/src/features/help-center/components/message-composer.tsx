import { FormEvent, useRef, useState } from 'react'
import { toast } from 'sonner'
import { MoreVertical, Paperclip, Send, Stethoscope, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useDiagnosticExport } from '@/hooks/help-center/useDiagnosticExport.ts'
import { MessageKind } from '@/schema/help-center.ts'
import { DiagnosticExportButton } from '@/features/help-center/components/diagnostic-export-button.tsx'

interface MessageComposerProps {
  isSending: boolean
  onSend: (body: string, file?: File, kind?: MessageKind) => void
  onTyping?: () => void
}

export function MessageComposer({ isSending, onSend, onTyping }: MessageComposerProps) {
  const [body, setBody] = useState('')
  const [pendingFile, setPendingFile] = useState<File | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const diagnosticExport = useDiagnosticExport((file) => onSend('', file, 'diagnostic'))

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!body.trim() && !pendingFile) return
    onSend(body.trim(), pendingFile ?? undefined, pendingFile ? 'attachment' : undefined)
    setBody('')
    setPendingFile(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (file.size > 25 * 1024 * 1024) {
      toast.error('حجم فایل از سقف ۲۵ مگابایت بیشتر است')
      return
    }
    setPendingFile(file)
  }

  return (
    <form onSubmit={handleSubmit} className='flex flex-col gap-2'>
      {pendingFile && (
        <div className='bg-muted flex items-center gap-2 rounded-md px-3 py-1.5 text-sm'>
          <Paperclip className='size-3.5 shrink-0' />
          <span className='truncate'>{pendingFile.name}</span>
          <button
            type='button'
            className='text-muted-foreground hover:text-foreground ml-auto'
            onClick={() => {
              setPendingFile(null)
              if (fileInputRef.current) fileInputRef.current.value = ''
            }}
          >
            <X className='size-3.5' />
          </button>
        </div>
      )}
      <div className='flex items-end gap-1.5 sm:gap-2'>
        <input ref={fileInputRef} type='file' className='hidden' onChange={handleFileChange} />

        {/* Desktop/tablet: two separate icon buttons, plenty of room.
            Mobile: collapsed into one "more actions" menu. */}
        <div className='hidden items-end gap-2 sm:flex'>
          <Button
            type='button'
            variant='outline'
            size='icon'
            onClick={() => fileInputRef.current?.click()}
            title='پیوست فایل'
          >
            <Paperclip className='size-4' />
          </Button>
          <DiagnosticExportButton onExported={(file) => onSend('', file, 'diagnostic')} />
        </div>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button type='button' variant='outline' size='icon' className='shrink-0 sm:hidden'>
              <MoreVertical className='size-4' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='start'>
            <DropdownMenuItem onClick={() => fileInputRef.current?.click()}>
              <Paperclip className='size-4' />
              پیوست فایل
            </DropdownMenuItem>
            <DropdownMenuItem onClick={diagnosticExport.runExport} disabled={diagnosticExport.isExporting}>
              <Stethoscope className='size-4' />
              ارسال گزارش تشخیصی
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <Textarea
          value={body}
          onChange={(e) => {
            setBody(e.target.value)
            onTyping?.()
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              handleSubmit(e)
            }
          }}
          placeholder='پیامی بنویسید...'
          rows={1}
          className='max-h-32 min-h-10 flex-1 resize-none'
        />
        <Button type='submit' size='icon' disabled={isSending || (!body.trim() && !pendingFile)}>
          <Send className='size-4' />
        </Button>
      </div>
    </form>
  )
}
