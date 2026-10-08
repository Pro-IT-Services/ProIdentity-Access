import { useEffect, useState } from 'react'
import { AlertCircle, Download, Languages, Loader2, Lock, LogOut, Server, ShieldCheck, RefreshCw, Trash2, User } from 'lucide-react'
import { useManagedStore } from '../stores/useManagedStore'
import { useUpdateStore } from '../stores/useUpdateStore'
import { Sheet } from './ui/Sheet'
import { Button } from './ui/Button'
import { UninstallApp } from '../../wailsjs/go/main/App'
import { t, LANGS, LANG_NAMES, getLangPreference, setLangPreference, type LangPreference } from '../i18n'

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
  const { settings, logout, loading, error, clearError } = useManagedStore()
  const [confirmUninstall, setConfirmUninstall] = useState(false)
  const [uninstalling, setUninstalling] = useState(false)
  const [langPref, setLangPref] = useState<LangPreference>(getLangPreference())
  const {
    status: update, checking: checkingUpdate, installing: installingUpdate,
    error: updateError, check: checkUpdate, install: installUpdate,
  } = useUpdateStore()

  useEffect(() => {
    if (!open) return
    setConfirmUninstall(false)
    clearError()
  }, [open, clearError])

  const handleUninstall = async () => {
    setUninstalling(true)
    try { await UninstallApp(true) } catch (e) { console.warn('uninstall failed', e); setUninstalling(false) }
  }

  return (
    <Sheet open={open} onClose={onClose} title={t('common.settings')} description={t('settings.desc')}>
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

        {/* Managed server — read-only; the URL is set during setup only. */}
        <Section title={t('settings.managedServer')} icon={Server}>
          <div className="space-y-1.5">
            <label className="block text-xs text-muted-foreground">{t('common.serverUrl')}</label>
            <div className="flex items-center gap-2 px-3 py-2.5 rounded-md border border-border bg-secondary/40">
              <span className="font-mono text-sm text-foreground break-all min-w-0">{settings.server_url || '—'}</span>
              <Lock className="w-3.5 h-3.5 shrink-0 text-muted-foreground/70 ml-auto" />
            </div>
            <p className="text-xs text-muted-foreground">{t('settings.serverLockedHint')}</p>
          </div>
          <div className="pt-1">
            <Button size="sm" variant="ghost" onClick={onReRunSetup}>
              <RefreshCw className="w-3.5 h-3.5" /> {t('settings.rerunWizard')}
            </Button>
          </div>
        </Section>

        {/* Account */}
        <Section title={t('settings.account')} icon={User}>
          {settings.logged_in ? (
            <>
              <Row label={t('settings.signedInAs')} value={
                <span className="font-medium">
                  {settings.username}
                  {settings.is_admin && (
                    <span className="ml-1.5 text-[10px] uppercase tracking-wider text-primary">{t('settings.admin')}</span>
                  )}
                </span>
              }/>
              {settings.vpn_name && <Row label={t('settings.vpn')} value={settings.vpn_name} />}
              <Row label={t('settings.twoFactor')} value={
                settings.totp_enabled
                  ? <span className="inline-flex items-center gap-1 text-success"><ShieldCheck className="w-3.5 h-3.5" /> {t('settings.enabled')}</span>
                  : <span className="text-muted-foreground">{t('settings.disabled')}</span>
              }/>
              <div className="pt-1">
                <Button size="sm" variant="outline" onClick={async () => { await logout(); }} disabled={loading}>
                  {loading ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <LogOut className="w-3.5 h-3.5" />}
                  {t('common.signOut')}
                </Button>
              </div>
            </>
          ) : (
            <>
              <p className="text-sm text-muted-foreground">{t('settings.notSignedIn')}</p>
              <Button size="sm" onClick={onSignIn} disabled={!settings.server_url}>
                {t('common.signIn')}
              </Button>
              {!settings.server_url && (
                <p className="text-xs text-muted-foreground">{t('settings.setUrlFirst')}</p>
              )}
            </>
          )}
        </Section>

        {/* Language */}
        <Section title={t('settings.language')} icon={Languages}>
          <select
            value={langPref}
            onChange={e => { const v = e.target.value as LangPreference; setLangPref(v); void setLangPreference(v) }}
            className="w-full h-9 rounded-md border border-border bg-secondary/40 px-3 text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="system">{t('settings.languageSystem')}</option>
            {LANGS.map(l => <option key={l} value={l}>{LANG_NAMES[l]}</option>)}
          </select>
        </Section>

        {/* Updates */}
        <Section title={t('settings.updates')} icon={Download}>
          <div className="space-y-1.5">
            <Row label={t('settings.installed')} value={update?.current_version || '—'} />
            {update?.latest_version && <Row label={t('settings.latest')} value={update.latest_version} />}
            {update && (
              <Row label={t('settings.status')} value={
                update.state === 'available' ? <span className="text-success">{t('update.stateAvailable')}</span>
                : update.state === 'downloading' ? <span>{t('update.stateDownloading', { progress: update.progress ?? 0 })}</span>
                : update.state === 'installing' ? <span>{t('update.stateInstalling')}</span>
                : update.state === 'failed' ? <span className="text-destructive">{t('update.stateFailed')}</span>
                : update.state === 'up_to_date' ? <span className="text-muted-foreground">{t('update.stateUpToDate')}</span>
                : <span className="text-muted-foreground">{t('update.stateNotChecked')}</span>
              } />
            )}
          </div>
          {!settings.server_url && (
            <p className="text-xs text-muted-foreground">{t('settings.updatesFromOrg')}</p>
          )}
          {updateError && <p className="text-xs text-destructive">{updateError}</p>}
          <div className="flex items-center gap-2 pt-1">
            <Button size="sm" variant="outline" onClick={() => checkUpdate()} disabled={!settings.server_url || checkingUpdate || installingUpdate}>
              {checkingUpdate ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
              {t('settings.check')}
            </Button>
            {update?.state === 'available' && (
              <Button size="sm" onClick={installUpdate} disabled={installingUpdate}>
                <Download className="w-3.5 h-3.5" /> {t('settings.installVersion', { version: update.latest_version ?? '' })}
              </Button>
            )}
          </div>
          <p className="text-xs text-muted-foreground">{t('settings.updatesByService')}</p>
        </Section>

        {/* Danger */}
        <Section title={t('settings.dangerZone')} icon={Trash2} dangerous>
          {!confirmUninstall ? (
            <>
              <p className="text-sm text-muted-foreground">
                {t('settings.uninstallDesc')}
              </p>
              <Button size="sm" variant="outline" className="text-destructive border-destructive/40 hover:bg-destructive/10" onClick={() => setConfirmUninstall(true)}>
                <Trash2 className="w-3.5 h-3.5" /> {t('settings.uninstallApp')}
              </Button>
            </>
          ) : (
            <>
              <p className="text-sm text-destructive">
                {t('settings.uninstallSure')}
              </p>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="ghost" onClick={() => setConfirmUninstall(false)} disabled={uninstalling}>{t('common.cancel')}</Button>
                <Button size="sm" variant="destructive" onClick={handleUninstall} disabled={uninstalling}>
                  {uninstalling
                    ? <><Loader2 className="w-3.5 h-3.5 animate-spin" /> {t('settings.uninstalling')}</>
                    : <><Trash2 className="w-3.5 h-3.5" /> {t('settings.confirmUninstall')}</>}
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
