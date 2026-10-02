import { StrictMode } from 'react'
import ReactDOM from 'react-dom/client'
import { AxiosError } from 'axios'
import {
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from '@tanstack/react-query'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { toast } from 'sonner'
import { ErrorBoundary } from '@/components/error-boundary'
import { useAuthStore } from '@/stores/authStore'
import { handleServerError } from '@/utils/handle-server-error'
import { FontProvider } from './context/font-context'
import { ThemeProvider } from './context/theme-context'
import './index.css'
// Vazirmatn is the only font in this app's font-switcher with real Persian
// glyph support -- Inter/Manrope (the two pre-existing options) silently
// fall back to the OS's own default Persian font for every Farsi string,
// which is the vast majority of this panel's text. Loaded globally (not
// per-feature like reports/v2ray-sub previously did) since font-context
// now defaults every user to it -- see config/fonts.ts.
import '@fontsource/vazirmatn/400.css'
import '@fontsource/vazirmatn/500.css'
import '@fontsource/vazirmatn/600.css'
import '@fontsource/vazirmatn/700.css'
import { routeTree } from './routeTree.gen'

export const router = createRouter({
  routeTree,
  context: {} as { queryClient: QueryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
})

// LICENSE_BLOCKED_ERROR_CODE mirrors the backend's
// middleware.LicenseBlockedErrorCode (see also
// components/license-lockdown-guard.tsx, which owns the real handling for
// this case). A 403 carrying this code means the install itself is
// unlicensed -- not "this specific user lacks permission" -- so it must
// NOT be treated like an ordinary 403 (which redirects to a generic
// "access denied" page). LicenseLockdownGuard is what actually renders
// the activation screen for this case; this handler only needs to get
// out of its way rather than force-navigating to /403 first.
const LICENSE_BLOCKED_ERROR_CODE = 'license_invalid'

// Global HTTP error handler
const handleGlobalHttpError = (error: unknown) => {
  if (error instanceof AxiosError) {
    const status = error.response?.status ?? 0
    const errorCode = error.response?.data?.error_code

    if (errorCode === LICENSE_BLOCKED_ERROR_CODE) {
      return
    }

    if (status === 401) {
      toast.error('Session expired!', { duration: 5000 })
      useAuthStore.getState().auth.reset()
      router.navigate({ to: '/sign-in' })
    }

    if (status === 403) {
      toast.error('Access denied!', { duration: 5000 })
      router.navigate({ to: '/403', replace: true })
    }
  }
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: import.meta.env.PROD,
      staleTime: 10 * 1000,
    },
    mutations: {
      onError: (error) => {
        handleGlobalHttpError(error)
        handleServerError(error)

        if (error instanceof AxiosError && error.response?.status === 304) {
          toast.error('Content not modified!', { duration: 5000 })
        }
      },
    },
  },
  queryCache: new QueryCache({
    onError: handleGlobalHttpError,
  }),
})

router.update({
  context: { queryClient },
})

const rootElement = document.getElementById('root')!
if (!rootElement.innerHTML) {
  const root = ReactDOM.createRoot(rootElement)
  root.render(
    <StrictMode>
      <ErrorBoundary>
        <QueryClientProvider client={queryClient}>
          <ThemeProvider defaultTheme='dark' storageKey='vite-ui-theme'>
            <FontProvider>
              <RouterProvider router={router} />
            </FontProvider>
          </ThemeProvider>
        </QueryClientProvider>
      </ErrorBoundary>
    </StrictMode>
  )
}
