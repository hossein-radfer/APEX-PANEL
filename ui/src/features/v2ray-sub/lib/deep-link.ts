/**
 * Builds the actual URL to navigate to for an app's "add subscription"
 * deep link -- on Android, wraps the bare custom-scheme URL (e.g.
 * "v2rayng://install-sub?url=...") into Chrome's own intent:// syntax,
 * since Chrome for Android does not reliably honor a bare custom scheme
 * triggered from a web page (a confirmed, well-documented gotcha; other
 * mobile browsers generally handle the bare scheme fine). Falls back to
 * the app store listing via intent://'s own S.browser_fallback_url= if
 * the app isn't installed, instead of silently doing nothing.
 *
 * On non-Android platforms (iOS/desktop), returns the bare custom-scheme
 * URL unchanged -- intent:// is Android/Chrome-specific syntax.
 */
export function buildDeepLinkUrl(
  bareSchemeUrl: string,
  androidPackageId: string | undefined,
  fallbackUrl: string,
  isAndroid: boolean
): string {
  if (!isAndroid || !androidPackageId) {
    return bareSchemeUrl
  }

  const schemeMatch = /^([a-z0-9.+-]+):\/\/(.*)$/i.exec(bareSchemeUrl)
  if (!schemeMatch) {
    return bareSchemeUrl
  }
  const [, scheme, rest] = schemeMatch

  return `intent://${rest}#Intent;scheme=${scheme};package=${androidPackageId};S.browser_fallback_url=${encodeURIComponent(fallbackUrl)};end`
}
