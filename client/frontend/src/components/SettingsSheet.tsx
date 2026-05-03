import { useEffect, useState } from 'react'
import { AlertCircle, Download, Loader2, LogOut, Server, ShieldCheck, RefreshCw, Trash2, User } from 'lucide-react'
import { useManagedStore } from '../stores/useManagedStore'
import { checkForUpdate, type UpdateCheckResult } from '../wailsbridge'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'
import { Input } from './ui/Input'
import { UninstallApp } from '../../wailsjs/go/main/App'

interface Props {
  open: boolean
  onClose: () => void
  /** Trigger the first-run wizard again (server URL + register flow). */
  onReRunSetup: () => void
  /** Open the login sheet. */
  onSignIn: () => void
}

/**
 * Single place for: managed-server URL, current login state, sign-out,
 * re-run the setup wizard, and uninstall.
 */
export function SettingsSheet({ open, onClose, onReRunSetup, onSignIn }: Props) {
  const { settings, saveServerURL, logout, loading, error, clearError } = useManagedStore()
  const [url, setUrl] = useState('')
  const [savingURL, setSavingURL] = useState(false)
  const [confirmUninstall, setConfirmUninstall] = useState(false)
  const [uninstalling, setUninstalling] = useState(false)
  const [checkingUpdate, setCheckingUpdate] = useState(false)
  const [updateInfo, setUpdateInfo] = useState<UpdateCheckResult | null>(null)
  const [updateError, setUpdateError] = useState('')

  useEffect(() => {
    if (!open) return
    setUrl(settings.server_url)
    setConfirmUninstall(false)
    setUpdateError('')
    clearError()
  }, [open, settings.server_url, clearError])

  const dirty = url.trim() !== settings.server_url
  const canSave = dirty && /^https?:\/\//i.test(url.trim())

  const handleSaveURL = async () => {
    setSavingURL(true)
    try { await saveServerURL(url.trim()) } catch { /* error in store */ }
    finally { setSavingURL(false) }
  }

  const handleUninstall = async () => {
    setUninstalling(true)
    try { await UninstallApp(true) } catch (e) { console.warn('uninstall failed', e); setUninstalling(false) }
  }

  const handleCheckUpdate = async () => {
    setCheckingUpdate(true)
    setUpdateError('')
    try {
      setUpdateInfo(await checkForUpdate())
    } catch (e: any) {
      setUpdateError(String(e?.message ?? e))
    } finally {
      setCheckingUpdate(false)
    }
  }

  return (
    <Sheet open={open} onClose={onClose} title="Settings" description="Server, account, and app management.">
      <div className="space-y-6">
        {error && (
          <div className="flex items-start gap-2 px-3 py-2.5 bg-destructive/10 border border-destructive/30 rounded-md">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-destructive" />
            <span className="text-sm text-destructive break-words">{error}</span>
          </div>
        )}
        {updateError && (
          <div className="flex items-start gap-2 px-3 py-2.5 bg-destructive/10 border border-destructive/30 rounded-md">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-destructive" />
            <span className="text-sm text-destructive break-words">{updateError}</span>
          </div>
        )}

        {/* Managed server */}
        <Section title="Managed server" icon={Server}>
          <div className="space-y-1.5">
            <label className="block text-xs text-muted-foreground">Server URL</label>
            <Input
              value={url}
              onChange={e => setUrl(e.target.value)}
              placeholder="https://vpn.example.com"
              disabled={savingURL}
            />
            <p className="text-xs text-muted-foreground">Base URL of the ProIdentity Access server.</p>
          </div>
          <div className="flex items-center gap-2 pt-1">
            <Button size="sm" onClick={handleSaveURL} disabled={!canSave || savingURL}>
              {savingURL ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> Saving…</> : 'Save URL'}
            </Button>
            <Button size="sm" variant="ghost" onClick={onReRunSetup}>
              <RefreshCw className="w-3.5 h-3.5" /> Re-run setup wizard
            </Button>
          </div>
        </Section>

        {/* Account */}
        <Section title="Account" icon={User}>
          {settings.logged_in ? (
            <>
              <Row label="Signed in as" value={
                <span className="font-medium">
                  {settings.username}
                  {settings.is_admin && (
                    <span className="ml-1.5 text-[10px] uppercase tracking-wider text-primary">admin</span>
                  )}
                </span>
              }/>
              {settings.vpn_name && <Row label="VPN" value={settings.vpn_name} />}
              <Row label="Two-factor" value={
                settings.totp_enabled
                  ? <span className="inline-flex items-center gap-1 text-success"><ShieldCheck className="w-3.5 h-3.5" /> enabled</span>
                  : <span className="text-muted-foreground">disabled</span>
              }/>
              <div className="pt-1">
                <Button size="sm" variant="outline" onClick={async () => { await logout(); }} disabled={loading}>
                  {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <LogOut className="w-3.5 h-3.5" />}
                  Sign out
                </Button>
              </div>
            </>
          ) : (
            <>
              <p className="text-sm text-muted-foreground">Not signed in.</p>
              <Button size="sm" onClick={onSignIn} disabled={!settings.server_url}>
                Sign in
              </Button>
              {!settings.server_url && (
                <p className="text-xs text-muted-foreground">Set the server URL above first.</p>
              )}
            </>
          )}
        </Section>

        {/* Updates */}
        <Section title="Updates" icon={Download}>
          {updateInfo ? (
            <div className="space-y-1.5">
              <Row label="Installed" value={updateInfo.current_version || 'unknown'} />
              <Row label="Latest" value={updateInfo.latest_version || 'none published'} />
              <Row label="Status" value={
                updateInfo.available
                  ? <span className="text-success">update available</span>
                  : <span className="text-muted-foreground">up to date</span>
              } />
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Check the connected server for a published Windows client update.</p>
          )}
          <div className="flex items-center gap-2 pt-1">
            <Button size="sm" variant="outline" onClick={handleCheckUpdate} disabled={!settings.server_url || checkingUpdate}>
              {checkingUpdate ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
              Check
            </Button>
          </div>
        </Section>

        {/* Danger */}
        <Section title="Danger zone" icon={Trash2} dangerous>
          {!confirmUninstall ? (
            <>
              <p className="text-sm text-muted-foreground">
                Removes the app, daemon service, and all local credentials. Requires admin rights.
              </p>
              <Button size="sm" variant="outline" className="text-destructive border-destructive/40 hover:bg-destructive/10" onClick={() => setConfirmUninstall(true)}>
                <Trash2 className="w-3.5 h-3.5" /> Uninstall ProIdentity Access
              </Button>
            </>
          ) : (
            <>
              <p className="text-sm text-destructive">
                Are you sure? This will run the uninstaller and quit the app.
              </p>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="ghost" onClick={() => setConfirmUninstall(false)} disabled={uninstalling}>Cancel</Button>
                <Button size="sm" variant="destructive" onClick={handleUninstall} disabled={uninstalling}>
                  {uninstalling
                    ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> Uninstalling…</>
                    : <><Trash2 className="w-3.5 h-3.5" /> Confirm uninstall</>}
                </Button>
              </div>
            </>
          )}
        </Section>
      </div>
    </Sheet>
  )
}

function Section({
  title, icon: Icon, dangerous, children,
}: { title: string; icon: React.ElementType; dangerous?: boolean; children: React.ReactNode }) {
  return (
    <div>
      <div className={`flex items-center gap-1.5 mb-2 text-[10px] uppercase tracking-wider font-semibold ${dangerous ? 'text-destructive/80' : 'text-muted-foreground'}`}>
        <Icon className="w-3 h-3" /> {title}
      </div>
      <div className="space-y-2.5">{children}</div>
    </div>
  )
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[120px_1fr] items-center gap-3 text-xs">
      <span className="text-muted-foreground">{label}</span>
      <div className="min-w-0 text-foreground">{value}</div>
    </div>
  )
}
