import { isAxiosError } from 'axios'

/**
 * Extracts a human-readable message from a caught error, preferring the
 * backend's own `message` field (see schema.ErrorResponse's `Message`
 * field in the Go backend) over a generic fallback.
 *
 * A confirmed, reported bug: several forms in this codebase caught any
 * mutation error and always showed the identical generic toast (e.g.
 * "Failed to save reseller. Please try again.") regardless of what the
 * backend actually returned -- including specific, actionable messages
 * like "username already exists". This made it impossible for an admin to
 * tell why a save failed even after filling in every field correctly.
 */
export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (isAxiosError(error)) {
    const message = error.response?.data?.message
    if (typeof message === 'string' && message.trim() !== '') {
      return message
    }
  }
  return fallback
}
