import { Component, type ReactNode } from 'react'

interface Props { children: ReactNode }
interface State { error: string | null }

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(e: unknown): State {
    return { error: e instanceof Error ? e.message : String(e) }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex flex-col items-center justify-center h-full gap-3 p-8 text-center">
          <p className="text-danger font-medium">Something went wrong</p>
          <p className="text-xs text-text-secondary font-mono break-all">{this.state.error}</p>
          <button
            onClick={() => this.setState({ error: null })}
            className="px-3 py-1.5 text-xs bg-bg-card border border-bg-border rounded-lg text-text-secondary hover:text-text-primary transition-colors"
          >
            Dismiss
          </button>
        </div>
      )
    }
    return this.props.children
  }
}
