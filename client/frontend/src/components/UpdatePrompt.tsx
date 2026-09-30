import { AlertTriangle, Download, Loader2, ShieldCheck } from 'lucide-react'
import { Button } from './ui/Button'
import { useUpdateStore } from '../stores/useUpdateStore'
import { t, tNode } from '../i18n'

/**
 * Asks the user to install a client update. The ProIdentity service installs
 * it with system rights, so this works without admin rights; the app closes
 * during installation and the service reopens it.
 */
export function UpdatePrompt() {
  const { status, error, installing, install, later } = useUpdateStore()
  const open = useUpdateStore(s => s.shouldPrompt())
  if (!open || !status) return null

  const busy = status.state === 'downloading' || status.state === 'installing' || installing
  const failed = status.state === 'failed' || (!!error && !busy)
  const mandatory = !!status.mandatory

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center p-6" role="alertdialog" aria-modal="true"
         aria-labelledby="update-title" aria-describedby="update-body">
      <div className="absolute inset-0 bg-black/55 backdrop-blur-sm animate-fade-in" />
      <div className="relative w-full max-w-sm rounded-xl border border-border bg-card shadow-2xl p-6 no-drag animate-fade-in">
        <div className="flex items-start gap-3">
          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full ${failed ? 'bg-destructive/15 text-destructive' : 'bg-primary/15 text-primary'}`}>
            {failed ? <AlertTriangle className="h-5 w-5" /> : busy ? <Loader2 className="h-5 w-5 animate-spin" /> : <Download className="h-5 w-5" />}
          </div>
          <div className="min-w-0">
            <h2 id="update-title" className="text-base font-semibold leading-tight">
              {failed ? t('update.failedTitle') : busy ? (status.state === 'installing' ? t('update.installingTitle') : t('update.downloadingTitle')) : t('update.availableTitle')}
            </h2>
            <p id="update-body" className="mt-1 text-sm text-muted-foreground">
              {failed
                ? (error || status.error || t('update.couldNotInstall'))
                : status.state === 'installing'
                  ? t('update.closingNow')
                  : busy
                    ? t('update.downloadingBody', { version: status.latest_version ?? '' })
                    : tNode('update.readyBody', { version: <span key="v" className="font-medium text-foreground">{status.latest_version}</span>, current: status.current_version })}
            </p>
          </div>
        </div>

        {status.state === 'downloading' && (
          <div className="mt-4 h-1.5 w-full overflow-hidden rounded-full bg-secondary" role="progressbar"
               aria-valuemin={0} aria-valuemax={100} aria-valuenow={status.progress ?? 0}>
            <div className="h-full rounded-full bg-primary transition-[width] duration-300" style={{ width: `${status.progress ?? 0}%` }} />
          </div>
        )}

        {!busy && !failed && (
          <>
            {status.notes && <p className="mt-4 text-sm">{status.notes}</p>}
            {mandatory && (
              <p className="mt-3 text-sm font-medium text-warning">{t('update.mandatory')}</p>
            )}
            <ul className="mt-4 space-y-1.5 text-xs text-muted-foreground">
              <li className="flex gap-2"><ShieldCheck className="h-3.5 w-3.5 shrink-0 text-success mt-0.5" />{t('update.bulletService')}</li>
              <li className="flex gap-2"><ShieldCheck className="h-3.5 w-3.5 shrink-0 text-success mt-0.5" />{t('update.bulletSigned')}</li>
              <li className="flex gap-2"><AlertTriangle className="h-3.5 w-3.5 shrink-0 text-warning mt-0.5" />{t('update.bulletDrop')}</li>
            </ul>
          </>
        )}

        {!busy && (
          <div className="mt-6 flex justify-end gap-2">
            {(!mandatory || failed) && (
              <Button variant="ghost" onClick={later}>{failed ? t('common.close') : t('update.later')}</Button>
            )}
            <Button onClick={install}>
              {failed ? t('common.tryAgain') : t('update.now')}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
