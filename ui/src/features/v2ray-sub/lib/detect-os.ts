export type DetectedOS = 'android' | 'ios' | 'windows' | 'linux' | 'macos' | 'unknown'

/**
 * Best-effort client OS detection from the browser's own User-Agent/
 * platform strings -- used only to auto-expand the matching platform
 * section in the apps accordion and label it "Your OS," never for any
 * security or content-negotiation decision (that happens server-side via
 * a real User-Agent check, see isV2RayClientRequest in the Go backend).
 */
export function detectOS(): DetectedOS {
  if (typeof navigator === 'undefined') return 'unknown'

  const ua = navigator.userAgent.toLowerCase()

  if (/android/.test(ua)) return 'android'
  if (/iphone|ipad|ipod/.test(ua)) return 'ios'
  if (/windows/.test(ua)) return 'windows'
  if (/mac os x|macintosh/.test(ua)) return 'macos'
  if (/linux/.test(ua)) return 'linux'

  return 'unknown'
}
