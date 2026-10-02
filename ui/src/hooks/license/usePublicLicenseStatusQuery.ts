import { useQuery } from '@tanstack/react-query'
import { fetchPublicLicenseStatus } from '@/api/license.ts'

// Polled at the root level (routes/__root.tsx) before any login attempt,
// to decide whether the entire app should be locked down to the
// activation screen. A short refetch interval matters here: if the
// license server later revokes/expires an install mid-session, this is
// what notices and re-locks the UI without requiring a manual refresh.
//
// retry:3 with backoff (not the old retry:1) -- LicenseLockdownGuard fails
// CLOSED (shows a "can't reach the panel" screen, not the real app) once
// this query's isError flips true, which is the right call for a genuine
// outage, but retry:1 (2 total attempts) meant two back-to-back transient
// blips -- a reverse-proxy hiccup, a brief backend restart window, an
// ordinary network jitter during a direct/hard page navigation -- was
// enough to trip it. Live-reproduced: two consecutive failures on a hard
// nav to a deep link left the user stuck on the lockdown screen until a
// manual reload happened to land outside the bad window. More attempts
// with backoff absorb exactly that class of blip while still failing
// closed for an actually-down backend/license server.
export const usePublicLicenseStatusQuery = () =>
  useQuery({
    queryKey: ['public_license_status'],
    queryFn: fetchPublicLicenseStatus,
    refetchInterval: 60_000,
    retry: 3,
    retryDelay: (attemptIndex) => Math.min(500 * 2 ** attemptIndex, 4_000),
  })
