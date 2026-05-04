import { useEffect, useState } from 'react'
import { useTunnelStore } from './stores/useTunnelStore'
import { useManagedStore } from './stores/useManagedStore'
import { useSetupStore } from './stores/useSetupStore'
import Header from './components/Header'
import Sidebar from './components/Sidebar'
import TunnelDetail from './components/TunnelDetail'
import ImportModal from './components/ImportModal'
import SetupWizard from './components/SetupWizard'
import ErrorBoundary from './components/ErrorBoundary'
import { onEvent } from './bridge'
import type { StatsInfo, TunnelInfo } from './types'

export default function App() {
  const { refresh, updateStats, updateTunnel, selectedId, setSelected } = useTunnelStore()
  const { loadSettings } = useManagedStore()
  const { setupDone, checkSetup } = useSetupStore()
  const [showImport, setShowImport] = useState(false)
  const [wizardFromSettings, setWizardFromSettings] = useState(false)
  const [setupChecked, setSetupChecked] = useState(false)

  useEffect(() => {
    checkSetup().finally(() => setSetupChecked(true))
  }, [])

  useEffect(() => {
    refresh()
    loadSettings()
    const interval = setInterval(refresh, 5000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    onEvent('tunnel.changed', (data: unknown) => {
      const info = data as TunnelInfo
      if (info) updateTunnel(info)
    })

    onEvent('stats.update', (data: unknown) => {
      const stats = data as StatsInfo
      if (stats) updateStats(stats)
    })

    onEvent('installation_revoked', () => {
      useTunnelStore.setState({ tunnels: [], selectedId: null, stats: {}, loading: false })
      useManagedStore.setState({
        servers: [],
        settings: { server_url: '', username: '', is_admin: false, logged_in: false, vpn_name: '', totp_enabled: false },
        loading: false,
        error: null,
      })
      useSetupStore.setState({ setupDone: false, step: 'mode', mode: null, serverURL: '', deviceName: '', loading: false, error: null })
    })
  }, [])

  if (!setupChecked) return null

  if (!setupDone) {
    const handleWizardClose = wizardFromSettings
      ? () => { setWizardFromSettings(false); useSetupStore.setState({ setupDone: true }) }
      : undefined
    return <SetupWizard onClose={handleWizardClose} />
  }

  const handleOpenWizard = () => {
    setWizardFromSettings(true)
    useSetupStore.setState({ setupDone: false, step: 'mode', mode: null, error: null })
  }

  return (
    <div className="flex flex-col h-full bg-bg-base">
      <Header onImport={() => setShowImport(true)} onSettings={handleOpenWizard} />

      <div className="flex-1 overflow-hidden flex flex-col">
        <ErrorBoundary>
          {selectedId ? (
            <TunnelDetail onBack={() => setSelected(null)} />
          ) : (
            <Sidebar onImport={() => setShowImport(true)} />
          )}
        </ErrorBoundary>
      </div>

      {showImport && <ImportModal onClose={() => setShowImport(false)} />}
    </div>
  )
}
