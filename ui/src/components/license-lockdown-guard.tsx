import { ReactNode } from 'react'
import { AxiosError } from 'axios'
import { Loader2 } from 'lucide-react'
import { usePublicLicenseStatusQuery } from '@/hooks/license/usePublicLicenseStatusQuery.ts'
import LicenseActivation from '@/features/auth/license-activation'
import LicenseCheckFailed from '@/components/license-check-failed.tsx'

interface Props {
  children: ReactNode
}

// LICENSE_BLOCKED_ERROR_CODE mirrors the backend's
// middleware.LicenseBlockedErrorCode -- when /license/public-status itself
// gets intercepted by LicenseMiddleware (see that endpoint's doc comment
// in api/http/license.go: this is deliberate, not a bug), the response is
// a 403 carrying this error_code rather than the normal 200 status
// payload. Checked explicitly so this specific, expected 403 can be told
// apart from a genuine network/connectivity failure -- the two must NOT
// be handled the same way (see isError branch below).
const LICENSE_BLOCKED_ERROR_CODE = 'license_invalid'

// Absolute lockdown: wraps the entire route tree (see routes/__root.tsx).
// Until the install is confirmed activated+valid (or still within its
// grace period after a temporary license-server outage -- see
// LicenseService.IsBlocking's exact rules on the backend), NOTHING else in
// the app is reachable -- no sign-in form, no dashboard shell, no public
// share links. This is deliberate and total: an admin who has never
// activated must see only the activation screen, with no way to browse
// around it, per this panel's licensing requirements.
export function LicenseLockdownGuard({ children }: Props) {
  const { data: status, isLoading, isError, error } = usePublicLicenseStatusQuery()

  // While the very first check is in flight, show a neutral loading state
  // rather than flashing the real app (or the activation screen) and then
  // possibly swapping to the other -- both would be jarring/wrong.
  if (isLoading) {
    return (
      <div className='bg-background flex h-svh w-full items-center justify-center'>
        <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
      </div>
    )
  }

  if (isError) {
    const axiosError = error instanceof AxiosError ? error : undefined

    // The backend's own doc comment on PublicStatus confirms this is
    // intentional: when the install is blocked, LicenseMiddleware
    // intercepts this exact endpoint and returns its 403 first, before
    // the handler ever runs -- "the frontend treats identically to an
    // unlicensed status." This branch is that treatment: show the
    // activation screen, exactly as if the (unreachable, in this case)
    // status payload had come back with activated=false.
    if (axiosError?.response?.data?.error_code === LICENSE_BLOCKED_ERROR_CODE) {
      return <LicenseActivation reason={axiosError.response.data.message} />
    }

    // Any OTHER failure -- a genuine network error, a timeout, a
    // malformed/unparsable response body (fetchPublicLicenseStatus's own
    // zod .parse() throws a ZodError here, not an AxiosError, so
    // axiosError is undefined for that case too) -- must FAIL CLOSED, not
    // open. This used to render `children` (the real app) on the theory
    // that "not license_invalid" meant "probably fine, and the backend's
    // own LicenseMiddleware still blocks real API calls anyway" -- but a
    // live reproduction proved that reasoning wrong: rendering `children`
    // means the full dashboard shell mounts with no session at all (no
    // login ever happened), and license_invalid 403s from every
    // subsequent dashboard query are explicitly ignored by the global
    // error handler (see main.tsx's handleGlobalHttpError skipping
    // LICENSE_BLOCKED_ERROR_CODE), so nothing ever resets or redirects --
    // exactly "no login screen, straight into the panel, Network Error on
    // everything" as reported live. Whether this endpoint is unreachable
    // because of a real outage or because of a transient proxy/DNS blip,
    // the only honest, safe response is to show a distinct
    // can't-reach-the-panel screen with a retry action -- never the
    // authenticated app.
    return <LicenseCheckFailed />
  }

  const isUnlocked = status?.activated && (status.valid || status.in_grace_period)

  if (!isUnlocked) {
    return <LicenseActivation reason={status?.reason} />
  }

  return <>{children}</>
}
