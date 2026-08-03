import { Component, type ErrorInfo, type ReactNode } from "react"

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary] Caught rendering error:", error, info.componentStack)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex min-h-screen items-center justify-center bg-background px-4">
          <div className="max-w-sm text-center space-y-4">
            <h1 className="text-xl font-bold text-foreground">Something went wrong</h1>
            <p className="text-sm text-muted-foreground">
              An unexpected error occurred. Try refreshing the page.
            </p>
            <button
              onClick={() => {
                this.setState({ hasError: false, error: null })
                window.location.reload()
              }}
              className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
            >
              Reload
            </button>
            {this.state.error && (
              <details className="mt-4 text-left">
                <summary className="text-xs text-muted-foreground cursor-pointer">
                  Error details
                </summary>
                <pre className="mt-2 text-xs text-red-500 overflow-auto max-h-40">
                  {this.state.error.message}
                </pre>
              </details>
            )}
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
