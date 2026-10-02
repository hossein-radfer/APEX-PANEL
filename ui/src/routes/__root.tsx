import { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { LicenseLockdownGuard } from '@/components/license-lockdown-guard.tsx'
import { Toaster } from '@/components/ui/sonner'
import { NavigationProgress } from '@/components/navigation-progress'
import GeneralError from '@/features/errors/general-error'
import NotFoundError from '@/features/errors/not-found-error'

export const Route = createRootRouteWithContext<{
  queryClient: QueryClient
}>()({
  component: () => {
    return (
      <>
        {/* Toaster must be mounted OUTSIDE LicenseLockdownGuard: while an
            install is unlicensed/never-activated, the guard renders
            LicenseActivation directly and never reaches its `children`
            prop -- so a Toaster nested inside children (as it was
            before) never mounts at all while the activation screen is
            showing, silently swallowing every toast.success/toast.error
            call from that screen's own forms (e.g. the Free Trial tab). */}
        <Toaster duration={50000} />
        <LicenseLockdownGuard>
          <NavigationProgress />
          <Outlet />
          {import.meta.env.MODE === 'development' && (
            <>
              <ReactQueryDevtools buttonPosition='bottom-left' />
              <TanStackRouterDevtools position='bottom-right' />
            </>
          )}
        </LicenseLockdownGuard>
      </>
    )
  },
  notFoundComponent: NotFoundError,
  errorComponent: GeneralError,
})
