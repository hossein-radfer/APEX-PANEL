// Shared, Persian-labeled formatting helpers for the Reports section. Every
// user-visible string this module produces is Farsi -- unit suffixes
// (بایت/کیلوبایت/...), the thousands separator via toLocaleString('fa-IR'),
// and Persian-digit dates -- per this section's "100% Persian" requirement.

// Confirmed, reported bug: v2ray's label here was 'وی‌توری', a
// mis-transliteration that doesn't correspond to how "V2Ray" is actually
// pronounced or written anywhere else in the panel (billing, accounting,
// resellers, dashboard all use the Latin brand name as-is, same as
// WireGuard). Kept in Latin here too rather than inventing another
// transliteration, matching the rest of the app.
const PROTOCOL_LABELS_FA: Record<string, string> = {
  wireguard: 'وایرگارد',
  user_manager: 'یوزر منیجر',
  v2ray: 'V2Ray',
}

export function protocolLabelFa(protocol: string | undefined | null): string {
  if (!protocol) return 'نامشخص'
  return PROTOCOL_LABELS_FA[protocol] ?? protocol
}

const BYTE_UNITS_FA = ['بایت', 'کیلوبایت', 'مگابایت', 'گیگابایت', 'ترابایت']

export function formatBytesFa(bytes: number | undefined | null): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) {
    return '۰ بایت'
  }
  if (bytes === 0) return '۰ بایت'
  const negative = bytes < 0
  let value = Math.abs(bytes)
  let unitIndex = 0
  while (value >= 1024 && unitIndex < BYTE_UNITS_FA.length - 1) {
    value /= 1024
    unitIndex++
  }
  const formatted = value.toLocaleString('fa-IR', {
    maximumFractionDigits: value >= 100 ? 0 : 1,
  })
  return `${negative ? '-' : ''}${formatted} ${BYTE_UNITS_FA[unitIndex]}`
}

// Renders a plain integer/float with Persian (Farsi) digits and thousands
// separators -- used for counts (users, packages, rows) rather than bytes.
export function formatNumberFa(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '۰'
  }
  return value.toLocaleString('fa-IR', { maximumFractionDigits: 1 })
}

export function formatPercentFa(value: number | undefined | null): string {
  if (value === undefined || value === null || Number.isNaN(value)) {
    return '٪۰'
  }
  return `٪${value.toLocaleString('fa-IR', { maximumFractionDigits: 1 })}`
}

// Formats an ISO date/timestamp string as a Persian (Jalali) short date via
// the built-in fa-IR-u-ca-persian locale calendar -- no extra date library
// needed, Intl already ships this in evergreen browsers.
export function formatDateFa(value: string | undefined | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('fa-IR-u-ca-persian', {
    month: 'short',
    day: 'numeric',
  }).format(date)
}

export function formatDateTimeFa(value: string | undefined | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('fa-IR-u-ca-persian', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function formatCurrencyFa(
  amount: number | undefined | null,
  currency?: string
): string {
  if (amount === undefined || amount === null || Number.isNaN(amount)) {
    return `۰ ${currency || 'تومان'}`
  }
  const formatted = amount.toLocaleString('fa-IR', {
    maximumFractionDigits: 0,
  })
  return `${formatted} ${currency || 'تومان'}`
}
