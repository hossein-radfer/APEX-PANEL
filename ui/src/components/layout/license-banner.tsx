import { IconAlertTriangle, IconRocket } from '@tabler/icons-react'
import { useLicenseStatusQuery } from '@/hooks/license/useLicenseStatusQuery.ts'
import { useUpdateCheckQuery } from '@/hooks/license/useUpdateCheckQuery.ts'
import { useAuthStore } from '@/stores/authStore.ts'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

// Admin-only: resellers never see license/update state, matching every
// other admin-only surface in this panel (Log Viewer, Backup, etc).
export function LicenseBanner() {
  const role = useAuthStore((state) => state.auth.admin?.role)
  const isAdmin = role === 'admin'

  const { data: license } = useLicenseStatusQuery(isAdmin)
  const { data: updateInfo } = useUpdateCheckQuery(isAdmin && !!license?.valid)

  if (!isAdmin || !license) return null

  if (!license.activated || (!license.valid && !license.in_grace_period)) {
    return (
      <Alert variant='destructive' className='rounded-none border-x-0 border-t-0'>
        <IconAlertTriangle className='h-4 w-4' />
        <AlertTitle>License Invalid</AlertTitle>
        <AlertDescription>
          {license.reason || 'This installation is not licensed.'} Contact your provider to
          resolve this before it stops working.
        </AlertDescription>
      </Alert>
    )
  }

  if (license.in_grace_period) {
    return (
      <Alert className='rounded-none border-x-0 border-t-0'>
        <IconAlertTriangle className='h-4 w-4' />
        <AlertTitle>License Server Unreachable</AlertTitle>
        <AlertDescription>
          Running on a temporary grace period. Restore connectivity to the license server soon.
        </AlertDescription>
      </Alert>
    )
  }

  if (updateInfo?.update_available) {
    return (
      <Alert
        variant={updateInfo.force_update ? 'destructive' : 'default'}
        className='rounded-none border-x-0 border-t-0'
      >
        <IconRocket className='h-4 w-4' />
        <AlertTitle>
          {updateInfo.force_update ? 'Mandatory Update Available' : 'Update Available'}: v
          {updateInfo.version}
        </AlertTitle>
        <AlertDescription>
          {updateInfo.change_log || 'A new version of the panel is available.'}
          {' '}See Settings &rarr; License for details.
        </AlertDescription>
      </Alert>
    )
  }

  return null
}
