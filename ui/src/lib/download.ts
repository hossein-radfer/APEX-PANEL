// Parses the filename out of a Content-Disposition response header (e.g.
// `attachment; filename="openvpn-certificate.pem"`), so a downloaded blob
// can be saved with the server-computed name/extension instead of a
// hardcoded guess. Requires the backend to expose this header via CORS
// (see http-server.go's ExposeHeaders) when the API is on a different
// origin than the frontend.
export function parseFilenameFromContentDisposition(
  headerValue: string | undefined
): string | null {
  if (!headerValue) return null
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(headerValue)
  if (!match) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}

export function triggerBlobDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

// A confirmed, reported bug: when the server-computed filename couldn't be
// recovered from Content-Disposition (see parseFilenameFromContentDisposition
// -- e.g. a proxy stripping response headers, or any other reason that
// header doesn't reach the browser), callers were falling back to a
// filename with NO extension at all (e.g. `${protocol}-client-app`). Saving
// a blob download with no extension is exactly what caused the reported
// symptom: the file the customer got was renamed to `.txt` regardless of
// what was actually uploaded (Windows/Chrome's own behavior for an
// extensionless blob save is to append `.txt`, since it can't otherwise
// infer a file type to associate). This derives a real extension from the
// blob's own MIME type as a second-best fallback -- correct far more often
// than no extension at all -- so admin-uploaded files (.exe, .apk, .zip,
// .ovpn, .p12, etc.) keep their real, usable extension end-to-end even in
// whatever environment caused the header to go missing.
const mimeToExtension: Record<string, string> = {
  'application/vnd.android.package-archive': '.apk',
  'application/x-msdownload': '.exe',
  'application/x-msi': '.msi',
  'application/zip': '.zip',
  'application/x-7z-compressed': '.7z',
  'application/x-rar-compressed': '.rar',
  'application/pdf': '.pdf',
  'application/x-pkcs12': '.p12',
  'application/x-x509-ca-cert': '.cer',
  'application/x-pem-file': '.pem',
  'application/octet-stream': '', // genuinely unknown -- no extension is better than a wrong one
}

export function extensionFromMimeType(mimeType: string | undefined): string {
  if (!mimeType) return ''
  const base = mimeType.split(';')[0].trim().toLowerCase()
  return mimeToExtension[base] ?? ''
}

// filenameWithFallbackExtension builds a download filename when the real,
// server-computed name (from Content-Disposition) isn't available -- see
// extensionFromMimeType's doc comment for why this exists. baseName should
// NOT already include an extension.
export function filenameWithFallbackExtension(
  baseName: string,
  blob: Blob
): string {
  return `${baseName}${extensionFromMimeType(blob.type)}`
}
