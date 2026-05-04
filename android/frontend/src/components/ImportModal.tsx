import { useState } from 'react'
import { X, Upload, FileText, AlertCircle, Loader2 } from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { pickFile } from '../bridge'

interface ImportModalProps {
  onClose: () => void
}

export default function ImportModal({ onClose }: ImportModalProps) {
  const { import: importTunnel } = useTunnelStore()
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handlePickFile = async () => {
    try {
      const result = await pickFile()
      if (result?.content) {
        setContent(result.content)
        if (!name && result.name) {
          setName(result.name.replace(/\.conf$/i, ''))
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      if (!msg.includes('cancelled')) {
        setError(msg || 'Could not open file')
      }
    }
  }

  const handleSubmit = async () => {
    const trimmedContent = content.trim()
    if (!trimmedContent) {
      setError('Please provide a WireGuard config')
      return
    }
    setLoading(true)
    setError('')
    try {
      await importTunnel(name.trim(), trimmedContent)
      onClose()
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e)
      setError(msg || 'Import failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 animate-fade-in"
      onClick={e => e.target === e.currentTarget && !loading && onClose()}
    >
      <div className="w-full max-w-lg bg-bg-card border border-bg-border rounded-2xl shadow-2xl animate-slide-up mx-4">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-bg-border">
          <h2 className="font-semibold text-text-primary">Import WireGuard Config</h2>
          <button
            onClick={onClose}
            disabled={loading}
            className="p-1.5 rounded-lg text-text-muted hover:text-text-secondary hover:bg-bg-hover transition-colors disabled:opacity-40"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Body */}
        <div className="px-5 py-4 space-y-4">
          {/* Name */}
          <div>
            <label className="block text-xs text-text-secondary mb-1.5">
              Tunnel name <span className="text-text-muted">(optional)</span>
            </label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="My VPN"
              disabled={loading}
              className="w-full px-3 py-2 bg-bg-surface border border-bg-border rounded-lg text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors disabled:opacity-50"
            />
          </div>

          {/* Config content */}
          <div>
            <label className="block text-xs text-text-secondary mb-1.5">Config</label>
            {content ? (
              <div className="relative rounded-xl border border-bg-border">
                <textarea
                  value={content}
                  onChange={e => setContent(e.target.value)}
                  spellCheck={false}
                  disabled={loading}
                  className="w-full h-48 px-4 py-3 bg-transparent text-xs font-mono text-text-secondary resize-none focus:outline-none selectable disabled:opacity-50"
                />
                {!loading && (
                  <button
                    onClick={() => setContent('')}
                    className="absolute top-2 right-2 p-1 rounded-md text-text-muted hover:text-text-secondary hover:bg-bg-hover transition-colors"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>
            ) : (
              <div className="rounded-xl border-2 border-dashed border-bg-border">
                <div className="flex flex-col items-center gap-3 py-10">
                  <div className="w-10 h-10 rounded-xl bg-bg-surface border border-bg-border flex items-center justify-center">
                    <Upload className="w-5 h-5 text-text-muted" />
                  </div>
                  <div className="text-center">
                    <p className="text-sm text-text-secondary">
                      Tap to pick a <span className="font-mono">.conf</span> file
                    </p>
                    <p className="text-xs text-text-muted mt-0.5">or paste config below</p>
                  </div>
                  <button
                    onClick={handlePickFile}
                    disabled={loading}
                    className="flex items-center gap-2 px-4 py-2 bg-bg-surface border border-bg-border rounded-lg text-sm text-text-secondary hover:text-text-primary transition-colors disabled:opacity-50"
                  >
                    <FileText className="w-4 h-4" />
                    Pick File
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Paste button */}
          {!content && !loading && (
            <button
              onClick={() => {
                navigator.clipboard.readText().then(text => {
                  if (text.includes('[Interface]')) {
                    setContent(text)
                  } else {
                    setError('Clipboard does not contain a WireGuard config')
                  }
                }).catch(() => setError('Cannot read clipboard'))
              }}
              className="flex items-center gap-2 text-xs text-text-muted hover:text-text-secondary transition-colors"
            >
              <FileText className="w-3.5 h-3.5" />
              Paste from clipboard
            </button>
          )}

          {/* Error */}
          {error && (
            <div className="flex items-start gap-2 px-3 py-2.5 bg-danger/10 border border-danger/20 rounded-lg">
              <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-danger" />
              <span className="text-sm text-danger break-words">{error}</span>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-end gap-2 px-5 py-4 border-t border-bg-border">
          <button
            onClick={onClose}
            disabled={loading}
            className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary transition-colors disabled:opacity-40"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={loading || !content.trim()}
            className="flex items-center gap-2 px-4 py-2 bg-accent hover:bg-accent-hover disabled:opacity-40 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
          >
            {loading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            {loading ? 'Importing…' : 'Import'}
          </button>
        </div>
      </div>
    </div>
  )
}
