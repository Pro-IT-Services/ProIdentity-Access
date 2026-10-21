import { useCallback, useEffect, useRef, useState } from 'react'
import { WindowSetTitle } from '../../wailsjs/runtime/runtime'
import { MonoChip } from './ui/MonoChip'
import { useOpenVPNStore, ovpnErrorText } from '../stores/useOpenVPNStore'
import { Trash2, ShieldCheck, Plus, FileKey, Building2 } from 'lucide-react'
import { ConnRow, RowTag } from './ConnRow'
import type { Status } from './ui/StatusDot'
import {
  managedListOpenVPNProfiles,
  managedConnectOpenVPN, managedDisconnectOpenVPN,
  importOpenVPNProfile, deleteLocalOpenVPNProfile,
  type OpenVPNProfileView, type OpenVPNStatus,
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

const isPassphraseError = (st?: OpenVPNStatus) => !!st?.error_code?.startsWith('key_passphrase')

/**
 * OpenVPN profiles and the one connect form that's open — shared by the
 * "ProIdentity Access" and "Imported" groups of the connections sheet.
 */
export function useOpenVPN() {
  const [profiles, setProfiles] = useState<OpenVPNProfileView[]>([])
  const [loaded, setLoaded] = useState(false)
  const [openForm, setOpenForm] = useState<string | null>(null)
  const sessions = useOpenVPNStore(s => s.sessions)
  const reloadSessions = useOpenVPNStore(s => s.load)

  const reload = useCallback(() => {
    Promise.all([
      managedListOpenVPNProfiles().then(setProfiles).catch(() => setProfiles([])),
      reloadSessions(),
    ]).finally(() => setLoaded(true))
  }, [reloadSessions])
  useEffect(() => { reload() }, [reload])

  // A connect that stopped for the key passphrase reopens its form to ask for it.
  const askedFor = useRef<Set<string>>(new Set())
  useEffect(() => {
    for (const [id, st] of Object.entries(sessions)) {
      if (!isPassphraseError(st)) { askedFor.current.delete(id); continue }
      if (askedFor.current.has(id)) continue
      askedFor.current.add(id)
      setOpenForm(cur => cur ?? id)
    }
  }, [sessions])

  return { profiles, loaded, reload, sessions, openForm, setOpenForm }
}

export const ovpnSessionId = sid

export function OpenVPNRow({ profile: p, session: st, formOpen, onToggleForm, onFormDone, onDeleted }: {
  profile: OpenVPNProfileView
  session?: OpenVPNStatus
  formOpen: boolean
  onToggleForm: () => void
  onFormDone: () => void
  onDeleted: () => void
}) {
  const reloadSessions = useOpenVPNStore(s => s.load)
  const [busy, setBusy] = useState(false)
  const status: Status = st?.status === 'connected' || st?.status === 'connecting' || st?.status === 'error' ? st.status : 'disconnected'
  const live = status === 'connected' || status === 'connecting'

  const disconnect = async () => {
    setBusy(true)
    try { await managedDisconnectOpenVPN(sid(p)); await reloadSessions() }
    catch (e: any) { toast(String(e?.message ?? e), 'warning', 6000) }
    finally { setBusy(false) }
  }

  const del = async () => {
    if (!confirm(t('ovpn.deleteConfirm', { name: p.name }))) return
    try { await deleteLocalOpenVPNProfile(p.id); onDeleted() } catch { /* ignore */ }
  }

  return (
    <ConnRow
      name={p.name}
      icon={p.source === 'local' ? FileKey : Building2}
      status={status}
      tags={<>
        <RowTag>{p.dev_type}</RowTag>
        {p.requires_totp && <RowTag tone="primary"><ShieldCheck className="w-3 h-3" /> TOTP</RowTag>}
      </>}
      busy={busy}
      live={live}
      onConnect={onToggleForm}
      onDisconnect={disconnect}
      cancelMode={formOpen}
      actions={p.source === 'local' && !live ? (
        <button onClick={e => { e.stopPropagation(); del() }} aria-label={t('common.delete')} title={t('common.delete')}
          className="h-8 w-8 inline-flex items-center justify-center rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors">
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      ) : undefined}
    >
      {status === 'error' && (
        <p className="text-[11px] text-destructive mt-1.5 break-words">{(st && ovpnErrorText(st)) || t('ovpn.connectionFailed')}</p>
      )}
      {status === 'connected' && (
        <p className="text-[11px] text-muted-foreground mt-1 font-mono tabular-nums">
          {st?.ip && <span className="mr-3">{st.ip}</span>}
          <span aria-label={t('mission.received')}>↓ {fmtBytes(st?.rx_bytes ?? 0)}</span>
          <span className="ml-3" aria-label={t('mission.sent')}>↑ {fmtBytes(st?.tx_bytes ?? 0)}</span>
        </p>
      )}
      {formOpen && !live && (
        <ConnectForm profile={p} lastError={st?.error_code} onDone={onFormDone} />
      )}
    </ConnRow>
  )
}

/** "Import .ovpn" link with its inline form. */
export function OpenVPNImport({ onImported }: { onImported: () => void }) {
  const [open, setOpen] = useState(false)
  return (
    <>
      <button onClick={() => setOpen(v => !v)}
        className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors">
        <Plus className="w-3 h-3" /> {t('ovpn.import')}
      </button>
      {open && <div className="w-full mt-2"><ImportForm onDone={() => { setOpen(false); onImported() }} /></div>}
    </>
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

function ConnectForm({ profile, lastError, onDone }: { profile: OpenVPNProfileView; lastError?: string; onDone: () => void }) {
  const [username, setUsername] = useState(profile.remembered_user ?? '')
  const [password, setPassword] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [totp, setTotp] = useState('')
  const [customIP, setCustomIP] = useState(profile.custom_ip ?? '')
  const [remember, setRemember] = useState(profile.has_saved_password || profile.has_saved_passphrase)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const needsCreds = profile.auth_user_pass
  const needsIP = profile.dev_type === 'tap' && profile.allow_custom_ip
  const autofill = profile.autofill_name || profile.name
  const totpRef = useRef<HTMLInputElement>(null)
  const passphraseRef = useRef<HTMLInputElement>(null)
  useAutofillTitle(autofill)

  // Key passphrase: shown when the profile is known to need one or the last
  // attempt asked for it; required unless a saved one exists and wasn't rejected.
  const askPassphrase = profile.key_encrypted || profile.has_saved_passphrase || !!lastError?.startsWith('key_passphrase')
  const passphraseSaved = profile.has_saved_passphrase && lastError !== 'key_passphrase_wrong'
  const needPassphrase = askPassphrase && !passphraseSaved

  // Put the cursor on the first thing still to type: the login, then the key
  // passphrase, then the TOTP code.
  const credsReady = !needsCreds || (!!profile.remembered_user && profile.has_saved_password)
  const focusPassphrase = credsReady && needPassphrase
  const focusTotp = profile.requires_totp && credsReady && !needPassphrase

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    // A filled-in login ends with Enter after the password; the passphrase or
    // code still has to be typed, so go there instead of failing.
    if (needPassphrase && !passphrase) { passphraseRef.current?.focus(); return }
    if (profile.requires_totp && !totp.trim()) { totpRef.current?.focus(); return }
    setBusy(true); setError('')
    try {
      await managedConnectOpenVPN(profile.id, profile.source, username, password, totp, customIP, passphrase, remember)
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
            placeholder={t('common.username')} className={inputCls} autoFocus={!focusTotp} />
          <input id="ovpn-password" name="password" type="password" autoComplete="current-password" aria-label={t('common.password')}
            value={password} onChange={e => setPassword(e.target.value)}
            placeholder={profile.has_saved_password ? t('ovpn.passwordSaved') : t('common.password')} className={inputCls} />
        </>
      )}
      {askPassphrase && (
        // Not a site login: keep password managers from filling it.
        <input ref={passphraseRef} id="ovpn-key-passphrase" name="key-passphrase" type="password" autoComplete="off"
          aria-label={t('ovpn.keyPassphrase')} value={passphrase} onChange={e => setPassphrase(e.target.value)}
          placeholder={passphraseSaved ? t('ovpn.keyPassphraseSaved') : t('ovpn.keyPassphrase')}
          className={inputCls} autoFocus={focusPassphrase} />
      )}
      {profile.requires_totp && (
        <input ref={totpRef} id="ovpn-otp" name="otp" autoComplete="one-time-code" aria-label={t('ovpn.totpCode')}
          value={totp} onChange={e => setTotp(e.target.value)} placeholder={t('ovpn.totpPlaceholder')} inputMode="numeric" className={inputCls} autoFocus={focusTotp} />
      )}
      {needsIP && (
        <input value={customIP} onChange={e => setCustomIP(e.target.value)} placeholder={t('ovpn.customIp')} className={`${inputCls} font-mono`} />
      )}
      {(needsCreds || askPassphrase) && (
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
