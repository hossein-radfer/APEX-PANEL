import { IconRefresh, IconWifiOff } from '@tabler/icons-react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import AuthLayout from '@/features/auth/auth-layout'

// Shown whenever LicenseLockdownGuard could not get a definitive
// activated/blocked answer from the backend (a network failure, a
// malformed/unparsable response, a timeout) -- deliberately distinct from
// LicenseActivation (which assumes the backend answered clearly with
// "not licensed" and offers a key/trial form to fix that). Submitting a
// license key here would be pointless: if the panel can't be reached to
// even ask its status, it can't be reached to activate either. The only
// safe, honest action is to retry, never to render the real app -- see
// LicenseLockdownGuard's own doc comment for why this replaced silently
// rendering the dashboard on any non-license-blocked error.
export default function LicenseCheckFailed() {
  return (
    <AuthLayout>
      <Card className='gap-4 shadow-lg'>
        <CardHeader>
          <div className='bg-destructive/10 text-destructive mx-auto mb-2 flex size-12 items-center justify-center rounded-full'>
            <IconWifiOff className='size-6' />
          </div>
          <CardTitle className='text-center text-lg tracking-tight'>
            Can&apos;t Reach the Panel
          </CardTitle>
          <CardDescription className='text-center'>
            The panel's own server could not be reached to confirm its
            license status. This is usually temporary -- check your
            connection and try again in a moment.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button className='w-full' onClick={() => window.location.reload()}>
            <IconRefresh className='mr-2 size-4' />
            Retry
          </Button>
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
