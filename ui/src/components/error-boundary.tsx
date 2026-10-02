import { Component, type ErrorInfo, type ReactNode } from 'react'
import GeneralError from '@/features/errors/general-error'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
}

/**
 * Top-level safety net: catches any otherwise-uncaught JavaScript error
 * thrown while rendering (or in a lifecycle method / effect cleanup) of any
 * component in the tree below it, and shows the existing "500" fallback UI
 * instead of leaving the user with a blank or half-crashed page.
 *
 * TanStack Router's `errorComponent` (see routes/__root.tsx) only catches
 * errors during a route's own render/loader phase -- an error thrown from
 * deep inside a mounted dialog's own effect (e.g. the clipboard crash this
 * was added for: `navigator.clipboard` is `undefined` on a plain-HTTP
 * origin, since browsers only expose it in a secure context, so calling
 * `.writeText` throws immediately) is a plain uncaught exception that
 * React itself unmounts the whole tree for, with nothing here to catch it
 * unless a real class-based Error Boundary wraps the tree -- which only a
 * class component can be, per React's API (there is no hook equivalent).
 *
 * This must only be relied on as a last resort, not a substitute for
 * fixing the actual bug (see lib/clipboard.ts for the specific fix this
 * was added alongside) -- an Error Boundary stops one bad component from
 * taking down the entire app, but the component itself still doesn't work
 * until its actual bug is fixed.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // eslint-disable-next-line no-console
    console.error('Uncaught error caught by top-level ErrorBoundary:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return <GeneralError />
    }

    return this.props.children
  }
}
