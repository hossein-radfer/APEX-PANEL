/**
 * Copies text to the clipboard, safely, regardless of whether the page is
 * served over a secure context (HTTPS/localhost) or plain HTTP.
 *
 * navigator.clipboard is only defined by browsers in a "secure context" --
 * on plain HTTP (a very common deployment for this panel, e.g. behind a
 * router's LAN IP with no TLS cert), `navigator.clipboard` is `undefined`,
 * and calling `.writeText` on it throws a TypeError that crashes the whole
 * React tree (no component here catches it), taking down completely
 * unrelated features (Edit/Delete/etc. dialogs) along with it since they
 * all render under the same tree.
 *
 * Falls back to the legacy `document.execCommand('copy')` approach (via a
 * temporary off-screen textarea), which still works without a secure
 * context. Never throws -- returns false instead, so callers can show a
 * "copy failed" message rather than crashing.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Fall through to the legacy fallback below -- some browsers expose
      // navigator.clipboard but still reject the call (e.g. missing
      // permission, or a secure-context edge case this check didn't catch).
    }
  }

  return legacyCopyToClipboard(text)
}

function legacyCopyToClipboard(text: string): boolean {
  if (typeof document === 'undefined') return false

  const textarea = document.createElement('textarea')
  textarea.value = text
  // Keep it out of the visible viewport and out of the tab order, but
  // still attached to the DOM (required for execCommand('copy') to find a
  // selection to copy from) and not `display: none` (some browsers refuse
  // to select/copy from a display:none element).
  textarea.style.position = 'fixed'
  textarea.style.top = '0'
  textarea.style.left = '0'
  textarea.style.width = '1px'
  textarea.style.height = '1px'
  textarea.style.padding = '0'
  textarea.style.border = 'none'
  textarea.style.outline = 'none'
  textarea.style.boxShadow = 'none'
  textarea.style.background = 'transparent'
  textarea.style.opacity = '0'
  textarea.setAttribute('readonly', '')

  document.body.appendChild(textarea)

  const previousActiveElement = document.activeElement as HTMLElement | null
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)

  let succeeded = false
  try {
    succeeded = document.execCommand('copy')
  } catch {
    succeeded = false
  }

  document.body.removeChild(textarea)
  previousActiveElement?.focus?.()

  return succeeded
}
