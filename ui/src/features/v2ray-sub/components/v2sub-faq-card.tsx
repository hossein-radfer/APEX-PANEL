import { useState } from 'react'
import { ChevronDownIcon, HelpCircleIcon, SearchIcon } from 'lucide-react'
import { v2rayShareFaq } from '@/features/share/lib/faq-content.ts'
import { useFaqSearch } from '@/features/shared-components/use-faq-search.ts'

// v2ray-sub has its own fixed dark theme independent of the panel's
// shadcn/Tailwind tokens (see v2ray-sub.css's own doc comment), so this
// reuses the SAME question set and matching logic as SmartFaq (phase 2
// item 8's "answer based on the actual content of the question, not just
// a fixed FAQ") but renders with plain v2sub-* classes instead of shadcn
// components, matching this page's own accordion cards.
export function V2SubFaqCard() {
  const { query, setQuery, filtered } = useFaqSearch(v2rayShareFaq)
  const [openIndex, setOpenIndex] = useState<number | null>(null)

  return (
    <div className='v2sub-card'>
      <div className='v2sub-header-title' style={{ marginBottom: 12 }}>
        سوالات متداول
      </div>

      <div className='v2sub-faq-search'>
        <SearchIcon size={15} className='v2sub-faq-search-icon' />
        <input
          type='text'
          className='v2sub-faq-search-input'
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setOpenIndex(null)
          }}
          placeholder='سوال خود را جستجو کنید...'
        />
      </div>

      {query.trim() !== '' && filtered.length === 0 ? (
        <div className='v2sub-faq-empty'>
          <HelpCircleIcon size={15} />
          <span>پاسخی برای این سوال یافت نشد. برای راهنمایی بیشتر با پشتیبانی خود در تماس باشید.</span>
        </div>
      ) : (
        <div style={{ marginTop: 4 }}>
          {filtered.map((item, index) => {
            const isOpen = openIndex === index
            return (
              <div className='v2sub-faq-item' key={item.question}>
                <button
                  type='button'
                  className='v2sub-faq-question'
                  onClick={() => setOpenIndex(isOpen ? null : index)}
                  aria-expanded={isOpen}
                >
                  <span>{item.question}</span>
                  <ChevronDownIcon
                    size={16}
                    className={`v2sub-accordion-chevron ${isOpen ? 'v2sub-open' : ''}`}
                  />
                </button>
                {isOpen && <div className='v2sub-faq-answer'>{item.answer}</div>}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
