import {
  ActivateLicenseRequest,
  LicenseStatus,
  LicenseStatusResponseSchema,
  PublicLicenseStatus,
  PublicLicenseStatusResponseSchema,
  StartTrialRequest,
  UpdateCheckResponseSchema,
  UpdateCheckResult,
} from '@/schema/license.ts'
import axiosInstance from '@/api/axios-instance.ts'

export const fetchLicenseStatus = async (): Promise<LicenseStatus> => {
  const { data } = await axiosInstance.get('/license/status')
  const parsed = LicenseStatusResponseSchema.parse(data)
  return parsed.data
}

// fetchPublicLicenseStatus hits the unauthenticated /license/public-status
// endpoint -- used by the root-level lockdown guard (routes/__root.tsx)
// before any login attempt, so it must not require a JWT.
export const fetchPublicLicenseStatus = async (): Promise<PublicLicenseStatus> => {
  const { data } = await axiosInstance.get('/license/public-status')
  const parsed = PublicLicenseStatusResponseSchema.parse(data)
  return parsed.data
}

export const checkForUpdate = async (channel = 'stable'): Promise<UpdateCheckResult> => {
  const { data } = await axiosInstance.get('/license/check-update', { params: { channel } })
  const parsed = UpdateCheckResponseSchema.parse(data)
  return parsed.data
}

// activateLicense is deliberately called with axiosInstance directly rather
// than requiring a prior login -- the backend endpoint it hits
// (POST /license/activate) is the one route exempted from both JWT auth and
// LicenseMiddleware's block, and instead verifies admin credentials inside
// the request body itself (see api/http/license.go's Activate handler doc
// comment for why: while unlicensed, login itself is blocked, so there is
// no other way for an admin to ever reach this).
export const activateLicense = async (
  req: ActivateLicenseRequest
): Promise<LicenseStatus> => {
  const { data } = await axiosInstance.post('/license/activate', req)
  const parsed = LicenseStatusResponseSchema.parse(data)
  return parsed.data
}

// startTrial hits POST /license/trial -- same no-JWT/LicenseMiddleware
// exemption and admin-credential-in-body pattern as activateLicense above,
// see that function's doc comment for why.
export const startTrial = async (
  req: StartTrialRequest
): Promise<LicenseStatus> => {
  const { data } = await axiosInstance.post('/license/trial', req)
  const parsed = LicenseStatusResponseSchema.parse(data)
  return parsed.data
}

// revokeLicense clears this install's stored license key so the
// activation screen reappears -- unlike activate/trial, this IS behind
// JWT auth (see api/http/license.go's Revoke handler doc comment): an
// already-activated install always has a session to call this from.
export const revokeLicense = async (): Promise<LicenseStatus> => {
  const { data } = await axiosInstance.post('/license/revoke')
  const parsed = LicenseStatusResponseSchema.parse(data)
  return parsed.data
}
