import { FormEvent, useEffect, useRef, useState } from 'react'
import { MessageKind, Ticket } from '@/schema/help-center.ts'
import { ArrowLeft, LifeBuoy, Plus, Send, Users } from 'lucide-react'
import { toast } from 'sonner'
import {
  useCreateTicketMutation,
  usePingTypingMutation,
  usePostTicketMessageMutation,
  useTicketMessagesQuery,
  useTicketsQuery,
  useTypingStatusQuery,
} from '@/hooks/help-center/useHelpCenter.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { EmptyState } from '@/components/ui/empty-state.tsx'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { MessageBubble } from '@/features/help-center/components/message-bubble.tsx'
import { MessageComposer } from '@/features/help-center/components/message-composer.tsx'
import { TypingIndicator } from '@/features/help-center/components/typing-indicator.tsx'

function ticketStatusLabelFa(status: Ticket['Status']): string {
  return status === 'open' ? 'باز' : 'بسته‌شده'
}

function TicketListItem({
  ticket,
  active,
  onClick,
}: {
  ticket: Ticket
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      onClick={onClick}
      className={`hover:bg-muted w-full rounded-lg border p-3 text-left transition-colors ${
        active ? 'bg-muted border-primary' : ''
      }`}
    >
      <div className='flex items-center justify-between gap-2'>
        <span className='truncate text-sm font-medium'>{ticket.Subject}</span>
        {ticket.CustomerUnreadCount > 0 && (
          <Badge className='shrink-0'>{ticket.CustomerUnreadCount}</Badge>
        )}
      </div>
      <div className='mt-1 flex items-center justify-between'>
        <span className='text-muted-foreground text-[11px]'>
          {new Date(ticket.LastMessageAt).toLocaleString()}
        </span>
        <Badge
          variant={ticket.Status === 'open' ? 'default' : 'secondary'}
          className='text-[10px]'
        >
          {ticketStatusLabelFa(ticket.Status)}
        </Badge>
      </div>
    </button>
  )
}

export default function HelpCenter() {
  const { data: tickets, isLoading: isListLoading } = useTicketsQuery()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const { data: thread, isLoading: isThreadLoading } =
    useTicketMessagesQuery(selectedId)
  const { data: isAdminTyping } = useTypingStatusQuery(selectedId)
  const postMessage = usePostTicketMessageMutation(selectedId ?? 0)
  const pingTyping = usePingTypingMutation(selectedId ?? 0)
  const createTicket = useCreateTicketMutation()
  const scrollRef = useRef<HTMLDivElement>(null)
  const typingThrottleRef = useRef(0)

  const [newTicketOpen, setNewTicketOpen] = useState(false)
  const [newSubject, setNewSubject] = useState('')
  const [newMessage, setNewMessage] = useState('')

  useEffect(() => {
    if (tickets && tickets.length > 0 && selectedId === null) {
      setSelectedId(tickets[0].ID)
    }
  }, [tickets, selectedId])

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight })
  }, [thread?.messages.length])

  const handleTyping = () => {
    const now = Date.now()
    if (now - typingThrottleRef.current < 2000) return
    typingThrottleRef.current = now
    pingTyping.mutate()
  }

  const handleSend = (body: string, file?: File, kind?: MessageKind) => {
    if (!selectedId) return
    postMessage.mutate(
      { body, file, kind },
      { onError: () => toast.error('ارسال پیام ناموفق بود') }
    )
  }

  const handleCreateTicket = (e: FormEvent) => {
    e.preventDefault()
    if (!newMessage.trim()) {
      toast.error('توضیح مشکل را وارد کنید')
      return
    }
    createTicket.mutate(
      {
        subject: newSubject.trim() || 'درخواست پشتیبانی',
        message: newMessage.trim(),
      },
      {
        onSuccess: (ticket) => {
          toast.success('تیکت پشتیبانی ایجاد شد')
          setNewTicketOpen(false)
          setNewSubject('')
          setNewMessage('')
          setSelectedId(ticket.ID)
        },
        onError: () => toast.error('ایجاد تیکت ناموفق بود'),
      }
    )
  }

  return (
    <>
      <Header fixed>
        <Search />
        <div className='ml-auto flex items-center space-x-4'>
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main fixed className='space-y-4 md:space-y-6'>
        <div className='flex items-center justify-between gap-3'>
          <div className='min-w-0'>
            <h2 className='text-xl font-bold tracking-tight md:text-2xl'>
              مرکز پشتیبانی
            </h2>
            <p className='text-muted-foreground hidden text-sm md:block'>
              مستقیماً با پشتیبانی گفتگو کنید -- فایل پیوست کنید، پیام صوتی
              ضبط کنید یا گزارش تشخیصی ارسال کنید.
            </p>
          </div>
          <Button
            onClick={() => setNewTicketOpen(true)}
            className='shrink-0 gap-2'
          >
            <Plus className='size-4' />
            <span className='hidden sm:inline'>تیکت جدید</span>
          </Button>
        </div>

        <div className='flex flex-wrap gap-2'>
          <a
            href='https://t.me/apexpanel_official'
            target='_blank'
            rel='noopener noreferrer'
          >
            <Button variant='outline' size='sm' className='gap-2'>
              <Send className='size-4' />
              کانال تلگرام ApexPanel
            </Button>
          </a>
          <a
            href='https://t.me/apexpanel_group'
            target='_blank'
            rel='noopener noreferrer'
          >
            <Button variant='outline' size='sm' className='gap-2'>
              <Users className='size-4' />
              گروه پشتیبانی تلگرام
            </Button>
          </a>
        </div>

        {/* Mobile: show exactly one pane at a time (list, or the selected
          thread with a back button) -- the old always-side-by-side grid
          squeezed both panes into unusable slivers below the md
          breakpoint, with no way to get back to the list once a ticket
          was open. Desktop (md+) keeps both panes visible side by side.
          Both panels use flex-1 min-h-0 to fill whatever space Main's own
          "fixed" flex-column layout actually gives them -- NOT a raw
          100vh/100dvh calc against the whole viewport (a confirmed,
          reported bug: this page previously rendered with no <Main>
          wrapper at all, so its cards had no flex-managed height to fill
          and fell back to a fragile 100vh-minus-a-fixed-rem-offset guess,
          which is wrong on any mobile browser whose address bar
          shows/hides dynamically -- the classic mobile-viewport-height
          gotcha). */}
        <div className='grid min-h-0 flex-1 gap-4 md:grid-cols-[320px_1fr]'>
          <Card
            className={`flex min-h-0 flex-col ${
              selectedId !== null ? 'hidden md:flex' : ''
            }`}
          >
            <CardContent className='flex min-h-0 flex-1 flex-col gap-2 overflow-hidden p-3'>
              {isListLoading ? (
                <div className='space-y-2 p-1'>
                  <Skeleton className='h-16 w-full rounded-lg' />
                  <Skeleton className='h-16 w-full rounded-lg' />
                  <Skeleton className='h-16 w-full rounded-lg' />
                </div>
              ) : !tickets?.length ? (
                <EmptyState
                  icon={<LifeBuoy className='size-8 opacity-60' />}
                  message='هنوز هیچ تیکت پشتیبانی‌ای وجود ندارد. در صورت نیاز یکی بسازید.'
                />
              ) : (
                <ScrollArea className='flex-1'>
                  <div className='flex flex-col gap-2 pr-2'>
                    {tickets.map((ticket) => (
                      <TicketListItem
                        key={ticket.ID}
                        ticket={ticket}
                        active={ticket.ID === selectedId}
                        onClick={() => setSelectedId(ticket.ID)}
                      />
                    ))}
                  </div>
                </ScrollArea>
              )}
            </CardContent>
          </Card>

          <Card
            className={`flex min-h-0 flex-col ${
              selectedId === null ? 'hidden md:flex' : ''
            }`}
          >
            <CardContent className='flex min-h-0 flex-1 flex-col p-0'>
              {!selectedId || !thread ? (
                <EmptyState
                  icon={<LifeBuoy className='size-8 opacity-60' />}
                  message={
                    isThreadLoading
                      ? 'در حال بارگذاری گفتگو...'
                      : 'یک تیکت را انتخاب کنید یا تیکت جدیدی بسازید.'
                  }
                />
              ) : (
                <>
                  <div className='flex items-center justify-between gap-2 border-b p-3 md:p-4'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='-ml-1 shrink-0 md:hidden'
                        onClick={() => setSelectedId(null)}
                      >
                        <ArrowLeft className='size-4' />
                      </Button>
                      <p className='truncate font-medium'>
                        {thread.ticket.Subject}
                      </p>
                    </div>
                    <Badge
                      variant={
                        thread.ticket.Status === 'open'
                          ? 'default'
                          : 'secondary'
                      }
                      className='shrink-0'
                    >
                      {ticketStatusLabelFa(thread.ticket.Status)}
                    </Badge>
                  </div>

                  <ScrollArea className='flex-1 p-3 md:p-4' ref={scrollRef}>
                    <div className='flex flex-col gap-4'>
                      {thread.messages.map((message) => (
                        <MessageBubble
                          key={message.ID}
                          message={message}
                          isOwn={message.Author === 'customer'}
                        />
                      ))}
                      {isAdminTyping && <TypingIndicator />}
                    </div>
                  </ScrollArea>

                  <div className='border-t p-2 md:p-3'>
                    <MessageComposer
                      isSending={postMessage.isPending}
                      onSend={handleSend}
                      onTyping={handleTyping}
                    />
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </div>

        <Dialog open={newTicketOpen} onOpenChange={setNewTicketOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>تیکت پشتیبانی جدید</DialogTitle>
            </DialogHeader>
            <form onSubmit={handleCreateTicket} className='grid gap-4'>
              <div className='grid gap-2'>
                <Label htmlFor='subject'>موضوع (اختیاری)</Label>
                <Input
                  id='subject'
                  placeholder='مثلاً: مشکل ورود به پنل'
                  value={newSubject}
                  onChange={(e) => setNewSubject(e.target.value)}
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='message'>مشکل خود را توضیح دهید</Label>
                <Textarea
                  id='message'
                  rows={5}
                  value={newMessage}
                  onChange={(e) => setNewMessage(e.target.value)}
                  required
                />
              </div>
              <DialogFooter>
                <Button type='submit' disabled={createTicket.isPending}>
                  {createTicket.isPending ? 'در حال ایجاد...' : 'ایجاد تیکت'}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </Main>
    </>
  )
}
