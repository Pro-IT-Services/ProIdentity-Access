import { useEffect, useRef, useState } from 'react'
import { WindowSetTitle } from '../../wailsjs/runtime/runtime'
import { MonoChip } from './ui/MonoChip'
import { useOpenVPNStore } from '../stores/useOpenVPNStore'
import { Loader2, Power, Trash2, ShieldCheck, Plus, Search } from 'lucide-react'
import {
  managedListOpenVPNProfiles,
  managedConnectOpenVPN, managedDisconnectOpenVPN,
  importOpenVPNProfile, deleteLocalOpenVPNProfile,
  type OpenVPNProfileView,
} from '../wailsbridge'
import { toast } from './ui/Toast'
import { fmtDecimal, t } from '../i18n'

const inputCls = 'w-full h-8 rounded-md border border-border bg-background px-2 text-sm'
const sid = (p: OpenVPNProfileView) => `${p.source}:${p.id}`

function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(u.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  return `${fmtDecimal(n / Math.pow(1024, i), i ? 1 : 0)} ${u[i]}`
}

export function OpenVPNPanel() {
  const [profiles, setProfiles] = useState<OpenVPNProfileView[]>([])
  const sessions = useOpenVPNStore(s => s.sessions)
  const reloadSessions = useOpenVPNStore(s => s.load)
  const [openForm, setOpenForm] = useState<string | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [loaded, setLoaded] = useState(false)

  const load = () => {
    Promise.all([
      managedListOpenVPNProfiles().then(setProfiles).catch(() => setProfiles([])),
      reloadSessions(),
    ]).finally(() => setLoaded(true))
  }
  useEffect(() => { load() }, [])

  const disconnect = async (p: OpenVPNProfileView) => {
    try { await managedDisconnectOpenVPN(sid(p)); reloadSessions() }
    catch (e: any) { toast(String(e?.message ?? e), 'warning', 6000) }
  }

  const del = async (p: OpenVPNProfileView) => {
    if (p.source !== 'local' || !confirm(t('ovpn.deleteConfirm', { name: p.name }))) return
    try { await deleteLocalOpenVPNProfile(p.id); load() } catch { /* ignore */ }
  }

  const hasProfiles = profiles.length > 0
  const q = query.trim().toLowerCase()
  const filtered = q ? profiles.filter(p => p.name.toLowerCase().includes(q)) : profiles

  // Nothing to show until we've loaded, and no section at all when the user has
  // no OpenVPN profiles — only the subtle import affordance remains reachable.
  if (!loaded) return null

  return (
    <div className="space-y-2">
      {hasProfiles && (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="px-4 py-2 border-b border-border bg-secondary/30 flex items-center justify-between">
            <span className="text-[11px] uppercase tracking-wider font-semibold text-muted-foreground">OpenVPN</span>
            <span className="text-[11px] text-muted-foreground">
              {q ? `${filtered.length} / ${profiles.length}` : t('common.countTotal', { count: profiles.length })}
            </span>
          </div>

          {profiles.length > 4 && (
            <div className="px-3 py-2 border-b border-border">
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground" />
                <input value={query} onChange={e => setQuery(e.target.value)} placeholder={t('ovpn.search')}
                  className="w-full h-8 rounded-md border border-border bg-background pl-8 pr-2 text-sm" />
              </div>
            </div>
          )}

          <div className="divide-y divide-border">
            {filtered.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">{t('ovpn.noMatches')}</p>
            ) : filtered.map(p => {
              const st = sessions[sid(p)]
              const connected = st?.status === 'connected'
              const connecting = st?.status === 'connecting'
              return (
                <div key={sid(p)} className="p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium truncate">{p.name}</p>
                      <div className="flex flex-wrap items-center gap-1.5 mt-1 text-[10px] text-muted-foreground">
                        <span className="uppercase">{p.dev_type}</span>
                        {p.requires_totp && <span className="inline-flex items-center gap-0.5 text-primary"><ShieldCheck className="w-3 h-3" /> TOTP</span>}
                        <span>{p.source === 'local' ? t('ovpn.sourceImported') : t('ovpn.sourceAssigned')}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {connected || connecting ? (
                        <button onClick={() => disconnect(p)}
                          className="inline-flex items-center gap-1 text-xs px-2 py-1 rounded-md text-destructive hover:bg-destructive/10">
                          {connecting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Power className="w-3.5 h-3.5" />}
                          {connecting ? t('ovpn.connecting') : t('common.disconnect')}
                        </button>
                      ) : (
                        <button onClick={() => setOpenForm(openForm === sid(p) ? null : sid(p))}
                          className="text-xs px-2.5 py-1 rounded-md bg-primary/10 text-primary hover:bg-primary/20">
                          {t('common.connect')}
                        </button>
                      )}
                      {p.source === 'local' && (
                        <button onClick={() => del(p)} aria-label={t('common.delete')} title={t('common.delete')} className="text-muted-foreground hover:text-destructive">
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                  {st?.status === 'error' && (
                    <p className="text-[11px] text-destructive mt-1.5 break-words">{st.error || t('ovpn.connectionFailed')}</p>
                  )}
                  {connected && st?.ip && (
                    <p className="text-[11px] text-success mt-1.5">{t('ovpn.connectedLine', { ip: st.ip, rx: fmtBytes(st.rx_bytes), tx: fmtBytes(st.tx_bytes) })}</p>
                  )}
                  {openForm === sid(p) && !connected && !connecting && (
                    <ConnectForm profile={p} onDone={() => { setOpenForm(null); reloadSessions() }} />
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      <div className="flex justify-end pt-0.5">
        <button onClick={() => setImportOpen(v => !v)}
          className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors">
          <Plus className="w-3 h-3" /> {t('ovpn.import')}
        </button>
      </div>
      {importOpen && <ImportForm onDone={() => { setImportOpen(false); load() }} />}
    </div>
  )
}

const APP_TITLE = 'ProIdentity Access'

function setWindowTitle(title: string) {
  try { WindowSetTitle(title) } catch { /* not running inside Wails */ }
}

/**
 * Password managers that fill desktop apps (RoboForm on Windows) match a saved
 * login by the program and its window title, e.g. `exe://ProIdentity Access/*Office VPN*`.
 * While a connect form is open the title carries the profile's autofill name.
 */
function useAutofillTitle(name: string) {
  useEffect(() => {
    if (!name) return
    setWindowTitle(`${name} - ${APP_TITLE}`)
    return () => setWindowTitle(APP_TITLE)
  }, [name])
}

function ConnectForm({ profile, onDone }: { profile: OpenVPNProfileView; onDone: () => void }) {
  const [username, setUsername] = useState(profile.remembered_user ?? '')
  const [password, setPassword] = useState('')
  const [totp, setTotp] = useState('')
  const [customIP, setCustomIP] = useState(profile.custom_ip ?? '')
  const [remember, setRemember] = useState(profile.has_saved_password)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const needsCreds = profile.auth_user_pass
  const needsIP = profile.dev_type === 'tap' && profile.allow_custom_ip
  const autofill = profile.autofill_name || profile.name
  const totpRef = useRef<HTMLInputElement>(null)
  useAutofillTitle(autofill)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    // A filled-in login ends with Enter after the password; the code still
    // has to be typed, so go there instead of failing.
    if (profile.requires_totp && !totp.trim()) { totpRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      await managedConnectOpenVPN(profile.id, profile.source, username, password, totp, customIP, remember)
      onDone()
    } catch (err: any) { setError(String(err?.message ?? err)) }
    finally { setBusy(false) }
  }

  return (
    <form onSubmit={submit} autoComplete="on" aria-label={t('ovpn.signInTo', { name: autofill })} className="mt-3 pt-3 border-t border-border space-y-2">
      {error && <p className="text-xs text-destructive">{error}</p>}
      {needsCreds && (
        <>
          <p className="flex items-center gap-1.5 text-[11px] text-muted-foreground" title={t('ovpn.autofillHint')}>
            {t('ovpn.autofillName')} <MonoChip value={autofill} bare />
          </p>
          <input id="ovpn-username" name="username" autoComplete="username" aria-label={t('common.username')}
            value={username} onChange={e => setUsername(e.target.value)}
            onFocus={e => e.currentTarget.select()}
            placeholder={t('common.username')} className={inputCls} autoFocus />
          <input id="ovpn-password" name="password" type="password" autoComplete="current-password" aria-label={t('common.password')}
            value={password} onChange={e => setPassword(e.target.value)}
            placeholder={profile.has_saved_password ? t('ovpn.passwordSaved') : t('common.password')} className={inputCls} />
        </>
      )}
      {profile.requires_totp && (
        <input ref={totpRef} id="ovpn-otp" name="otp" autoComplete="one-time-code" aria-label={t('ovpn.totpCode')}
          value={totp} onChange={e => setTotp(e.target.value)} placeholder={t('ovpn.totpPlaceholder')} inputMode="numeric" className={inputCls} />
      )}
      {needsIP && (
        <input value={customIP} onChange={e => setCustomIP(e.target.value)} placeholder={t('ovpn.customIp')} className={`${inputCls} font-mono`} />
      )}
      {needsCreds && (
        <label className="flex items-center gap-2 text-xs text-muted-foreground">
          <input type="checkbox" checked={remember} onChange={e => setRemember(e.target.checked)} /> {t('ovpn.remember')}
        </label>
      )}
      <button type="submit" disabled={busy}
        className="w-full text-xs py-1.5 rounded-md bg-primary text-white hover:bg-primary/90 disabled:opacity-60">
        {busy ? t('common.connectingEllipsis') : t('common.connect')}
      </button>
    </form>
  )
}

function ImportForm({ onDone }: { onDone: () => void }) {
  const [name, setName] = useState('')
  const [config, setConfig] = useState('')
  const [requiresTotp, setRequiresTotp] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const onFile = async (f: File | null) => {
    if (!f) return
    setConfig(await f.text())
    if (!name) setName(f.name.replace(/\.ovpn$/i, ''))
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!config) { setError(t('ovpn.chooseFile')); return }
    setBusy(true); setError('')
    try { await importOpenVPNProfile(name || t('ovpn.defaultName'), config, requiresTotp); onDone() }
    catch (err: any) { setError(String(err?.message ?? err)) }
    finally { setBusy(false) }
  }

  return (
    <form onSubmit={submit} className="rounded-lg border border-border bg-card/40 p-3 space-y-2">
      {error && <p className="text-xs text-destructive">{error}</p>}
      <input type="file" accept=".ovpn,.conf" onChange={e => onFile(e.target.files?.[0] ?? null)} className="text-xs" />
      <input value={name} onChange={e => setName(e.target.value)} placeholder={t('ovpn.displayName')} aria-label={t('ovpn.displayName')} className={inputCls} />
      <label className="flex items-center gap-2 text-xs text-muted-foreground">
        <input type="checkbox" checked={requiresTotp} onChange={e => setRequiresTotp(e.target.checked)} /> {t('ovpn.requiresTotp')}
      </label>
      <button type="submit" disabled={busy}
        className="w-full text-xs py-1.5 rounded-md bg-primary text-white hover:bg-primary/90 disabled:opacity-60">
        {busy ? t('common.importing') : t('common.import')}
      </button>
    </form>
  )
}
