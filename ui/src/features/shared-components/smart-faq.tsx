import { IconHelpCircle, IconSearch } from '@tabler/icons-react'
import { FaqAccordion, FaqItem } from '@/features/shared-components/faq-accordion.tsx'
import { useFaqSearch } from '@/features/shared-components/use-faq-search.ts'
import { Input } from '@/components/ui/input'

interface Props {
  items: FaqItem[]
  title?: string
}

export function SmartFaq({ items, title = 'سوالات متداول' }: Props) {
  const { query, setQuery, filtered } = useFaqSearch(items)

  if (items.length === 0) return null

  return (
    <div className='space-y-3'>
      <div className='text-muted-foreground flex items-center justify-center gap-2 text-sm font-medium'>
        <IconHelpCircle className='h-4 w-4' />
        <span>{title}</span>
      </div>

      <div className='relative'>
        <IconSearch className='text-muted-foreground pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2' />
        <Input
          type='text'
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder='سوال خود را جستجو کنید...'
          className='pr-9'
        />
      </div>

      {query.trim() !== '' && filtered.length === 0 ? (
        <div className='text-muted-foreground flex items-center justify-center gap-2 rounded-lg border py-6 text-sm'>
          <IconHelpCircle className='h-4 w-4' />
          <span>پاسخی برای این سوال یافت نشد. برای راهنمایی بیشتر با پشتیبانی خود در تماس باشید.</span>
        </div>
      ) : (
        <FaqAccordion items={filtered} hideTitle />
      )}
    </div>
  )
}
