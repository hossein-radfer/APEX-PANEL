import { useMemo, useState } from 'react'
import { FaqItem } from '@/features/shared-components/faq-accordion.tsx'

// normalize makes Persian/Arabic keyboard variants of the same letter (ي/ی,
// ك/ک) and diacritics match each other, and strips punctuation -- without
// this, a search for "كانفيگ" (Arabic ك/ي) would never match FAQ content
// written with "کانفیگ" (Persian ک/ی), which is the single most common
// real-world typing variance for Persian-speaking VPN end users switching
// between phone keyboards.
function normalize(text: string): string {
  return text
    .replace(/[ي]/g, 'ی') // Arabic ي -> Persian ی
    .replace(/[ك]/g, 'ک') // Arabic ك -> Persian ک
    .replace(/[ً-ٰٟ]/g, '') // strip Arabic diacritics
    .replace(/[^\p{L}\p{N}\s]/gu, ' ') // strip punctuation
    .toLowerCase()
    .trim()
}

function tokenize(text: string): string[] {
  return normalize(text)
    .split(/\s+/)
    .filter((word) => word.length >= 2)
}

interface ScoredItem {
  item: FaqItem
  score: number
}

// scoreItem ranks an FAQ item against the visitor's own search words --
// this is the "answer based on the actual content of the question, not
// just a fixed FAQ" behavior (phase 2 item 8), implemented as
// zero-dependency keyword matching against the SAME static FAQ content
// (see faq-content.ts) rather than a real LLM call: every share page this
// feeds is fully public with no login and no rate-limiting anywhere in
// this codebase (see api/http/middleware -- no throttle exists), so wiring
// in a paid external AI API here would be an open-ended cost/abuse surface
// with no existing primitive to contain it. A question word appearing in
// the FAQ's own question scores higher than the same word only appearing
// in the answer, since the question text is what a visitor is actually
// trying to match.
function scoreItem(item: FaqItem, queryWords: string[]): number {
  const questionWords = new Set(tokenize(item.question))
  const answerWords = new Set(tokenize(item.answer))

  let score = 0
  for (const word of queryWords) {
    if (questionWords.has(word)) score += 3
    else if (answerWords.has(word)) score += 1
  }
  return score
}

// Shared search/ranking behind both FAQ presentations: the shadcn-themed
// SmartFaq (used on the four shadcn-themed share pages) and the custom
// dark-themed v2sub FAQ card (v2ray-sub has its own fixed visual language,
// independent of the panel's light/dark theme -- see v2ray-sub.css's own
// doc comment). Keeping the matching logic in one hook means the two
// presentations can never drift into scoring questions differently.
export function useFaqSearch(items: FaqItem[]) {
  const [query, setQuery] = useState('')

  const filtered = useMemo(() => {
    const queryWords = tokenize(query)
    if (queryWords.length === 0) return items

    const scored: ScoredItem[] = items
      .map((item) => ({ item, score: scoreItem(item, queryWords) }))
      .filter((entry) => entry.score > 0)
      .sort((a, b) => b.score - a.score)

    return scored.map((entry) => entry.item)
  }, [items, query])

  return { query, setQuery, filtered }
}
