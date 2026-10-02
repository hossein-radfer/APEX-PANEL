/**
 * Best-effort country-flag emoji for a location's title -- MWPanel has no
 * structured "country" field on a panel/location anywhere in this
 * codebase (only a free-text sale title, e.g. "Germany 1" or "آلمان ۱"),
 * so this is a simple keyword match over common country names in both
 * English and Persian. Falls back to a generic globe emoji when nothing
 * matches -- this is purely decorative, never blocks rendering.
 */
const COUNTRY_KEYWORDS: Array<[RegExp, string]> = [
  [/german|آلمان/i, '🇩🇪'],
  [/nether|holland|هلند/i, '🇳🇱'],
  [/france|french|فرانسه/i, '🇫🇷'],
  [/finland|فنلاند/i, '🇫🇮'],
  [/turk|ترکیه/i, '🇹🇷'],
  [/uk|britain|england|انگلیس|انگلستان/i, '🇬🇧'],
  [/us\b|usa|united states|امریکا|آمریکا/i, '🇺🇸'],
  [/canada|کانادا/i, '🇨🇦'],
  [/japan|ژاپن/i, '🇯🇵'],
  [/singapore|سنگاپور/i, '🇸🇬'],
  [/iran|ایران/i, '🇮🇷'],
  [/uae|dubai|امارات|دبی/i, '🇦🇪'],
  [/russia|روسیه/i, '🇷🇺'],
  [/poland|لهستان/i, '🇵🇱'],
  [/italy|ایتالیا/i, '🇮🇹'],
  [/spain|اسپانیا/i, '🇪🇸'],
  [/sweden|سوئد/i, '🇸🇪'],
  [/switzerland|سوئیس/i, '🇨🇭'],
  [/austria|اتریش/i, '🇦🇹'],
  [/india|هند/i, '🇮🇳'],
  [/hong ?kong|هنگ ?کنگ/i, '🇭🇰'],
  [/south ?korea|korea|کره/i, '🇰🇷'],
]

export function countryFlagFor(title: string): string {
  for (const [pattern, flag] of COUNTRY_KEYWORDS) {
    if (pattern.test(title)) return flag
  }
  return '🌐'
}
