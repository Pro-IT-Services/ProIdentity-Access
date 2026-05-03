import { useState, useRef } from 'react'
import { Upload, FileText, AlertCircle, Loader2, X } from 'lucide-react'
import { useTunnelStore } from '../stores/useTunnelStore'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'
import { Input } from './ui/Input'

export function ImportSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { import: importTunnel } = useTunnelStore()
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [dragging, setDragging] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const reset = () => { setName(''); setContent(''); setError(''); setLoading(false); setDragging(false) }

  const handleFile = (file: File) => {
    const defaultName = file.name.replace(/\.conf$/i, '')
    if (!name) setName(defaultName)
    file.text().then(setContent)
  }

  const submit = async () => {
    if (!content.trim()) { setError('Please provide a WireGuard config'); return }
    setLoading(true); setError('')
    try {
      await importTunnel(name.trim(), content.trim())
      reset()
      onClose()
    } catch (e: any) {
      setError(e?.message ?? String(e))
    } finally { setLoading(false) }
  }

  return (
    <Sheet
      open={open}
      onClose={() => { if (!loading) { reset(); onClose() } }}
      title="Import WireGuard config"
      description="Drop a .conf file or paste its contents."
      footer={
        <>
          <Button variant="ghost" onClick={() => { reset(); onClose() }} disabled={loading}>Cancel</Button>
          <Button onClick={submit} disabled={loading || !content.trim()}>
            {loading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            {loading ? 'Importing…' : 'Import'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="space-y-1.5">
          <label className="block text-xs text-muted-foreground">
            Tunnel name <span className="text-muted-foreground/70">(optional — defaults to filename)</span>
          </label>
          <Input value={name} onChange={e => setName(e.target.value)} placeholder="My VPN" disabled={loading} />
        </div>

        <div className="space-y-1.5">
          <label className="block text-xs text-muted-foreground">Config</label>
          <div
            onDragOver={e => { e.preventDefault(); setDragging(true) }}
            onDragLeave={() => setDragging(false)}
            onDrop={e => { e.preventDefault(); setDragging(false); const f = e.dataTransfer.files[0]; if (f) handleFile(f) }}
            onClick={() => !content && !loading && fileRef.current?.click()}
            className={`relative rounded-lg border-2 border-dashed transition-colors ${
              dragging ? 'border-primary bg-primary/5'
              : content ? 'border-border cursor-default'
              : 'border-border hover:border-primary/40 cursor-pointer'
            }`}
          >
            {content ? (
              <div className="relative">
                <textarea
                  value={content}
                  onChange={e => setContent(e.target.value)}
                  spellCheck={false}
                  disabled={loading}
                  className="w-full h-56 px-3 py-2.5 bg-transparent text-xs font-mono text-foreground/85 resize-none focus:outline-none selectable disabled:opacity-50"
                />
                {!loading && (
                  <button
                    onClick={e => { e.stopPropagation(); setContent('') }}
                    className="absolute top-2 right-2 p-1 rounded text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>
            ) : (
              <div className="flex flex-col items-center gap-3 py-12">
                <div className="w-10 h-10 rounded-md bg-secondary border border-border flex items-center justify-center">
                  <Upload className="w-5 h-5 text-muted-foreground" />
                </div>
                <div className="text-center">
                  <p className="text-sm text-foreground">Drop a <span className="font-mono">.conf</span> file here</p>
                  <p className="text-xs text-muted-foreground mt-0.5">or click to browse</p>
                </div>
              </div>
            )}
          </div>
        </div>

        {!content && !loading && (
          <button
            onClick={() => navigator.clipboard.readText().then(text => {
              if (text.includes('[Interface]')) setContent(text)
              else setError('Clipboard doesn\'t contain a WireGuard config')
            }).catch(() => setError('Cannot read clipboard'))}
            className="inline-flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
          >
            <FileText className="w-3.5 h-3.5" /> Paste from clipboard
          </button>
        )}

        {error && (
          <div className="flex items-start gap-2 px-3 py-2.5 bg-destructive/10 border border-destructive/30 rounded-md">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-destructive" />
            <span className="text-sm text-destructive break-words">{error}</span>
          </div>
        )}

        <input
          ref={fileRef}
          type="file"
          accept=".conf"
          className="hidden"
          onChange={e => { const f = e.target.files?.[0]; if (f) handleFile(f); e.target.value = '' }}
        />
      </div>
    </Sheet>
  )
}
