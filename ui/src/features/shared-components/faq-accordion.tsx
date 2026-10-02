import { useState } from 'react'
import { IconChevronDown, IconHelpCircle } from '@tabler/icons-react'
import { cn } from '@/lib/utils'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

export type FaqItem = {
  question: string
  answer: string
}

// Static, hardcoded FAQ content (category 2 item 4) -- there is no
// AI/LLM API key or config anywhere in this project (checked api/ for any
// OPENAI_API_KEY/ANTHROPIC_API_KEY/LLM config, found none), so rather than
// block this feature on an unprovisioned external AI dependency, this is a
// well-organized static FAQ covering the questions a share-page visitor
// (a peer/account/package's actual end user, not an admin) most commonly
// has. Only ONE item is expanded at a time -- a plain array index, not a
// Set, since these pages never need more than one open simultaneously.
export function FaqAccordion({
  items,
  title = 'سوالات متداول',
  hideTitle = false,
}: {
  items: FaqItem[]
  title?: string
  hideTitle?: boolean
}) {
  const [openIndex, setOpenIndex] = useState<number | null>(null)

  if (items.length === 0) return null

  return (
    <div className='space-y-3'>
      {!hideTitle && (
        <div className='text-muted-foreground flex items-center justify-center gap-2 text-sm font-medium'>
          <IconHelpCircle className='h-4 w-4' />
          <span>{title}</span>
        </div>
      )}
      <div className='divide-border bg-card divide-y rounded-lg border'>
        {items.map((item, index) => {
          const isOpen = openIndex === index
          return (
            <Collapsible
              key={index}
              open={isOpen}
              onOpenChange={(open) => setOpenIndex(open ? index : null)}
            >
              <CollapsibleTrigger asChild>
                <button
                  type='button'
                  className='flex w-full items-center justify-between gap-3 px-4 py-3 text-right text-sm font-medium'
                >
                  <span>{item.question}</span>
                  <IconChevronDown
                    className={cn(
                      'h-4 w-4 shrink-0 transition-transform duration-200',
                      isOpen && 'rotate-180'
                    )}
                  />
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <p className='text-muted-foreground px-4 pb-3 text-sm leading-6'>
                  {item.answer}
                </p>
              </CollapsibleContent>
            </Collapsible>
          )
        })}
      </div>
    </div>
  )
}
