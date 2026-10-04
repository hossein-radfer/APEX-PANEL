import { Component, type ErrorInfo, type ReactNode } from 'react'
import { IconAlertTriangle } from '@tabler/icons-react'
import { Card, CardContent } from '@/components/ui/card'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
}

/**
 * Card-scoped counterpart to the app-wide `ErrorBoundary`
 * (components/error-boundary.tsx). That one catches an uncaught error
 * anywhere in the tree and replaces the ENTIRE app with a full-page "500"
 * fallback -- correct as a last resort, but too large a blast radius for a
 * dashboard/security page made of many independent cards: one card's bug
 * (a chart library edge case, an unexpected API shape, etc.) would take
 * down every other card on the page along with it.
 *
 * Wrapping each card individually in this boundary instead means a single
 * card's crash shows a small in-place fallback while its siblings keep
 * working normally.
 */
export class CardErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    // eslint-disable-next-line no-console
    console.error('Uncaught error caught by CardErrorBoundary:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        <Card>
          <CardContent className='flex items-center gap-2 py-6 text-sm text-muted-foreground'>
            <IconAlertTriangle className='h-4 w-4 shrink-0' />
            <span>خطا در بارگذاری این بخش</span>
          </CardContent>
        </Card>
      )
    }

    return this.props.children
  }
}
