package com.proitservices.proidentity.access.ui.screen

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.ui.component.ImportModal
import com.proitservices.proidentity.access.ui.component.ManagedLoginModal
import com.proitservices.proidentity.access.ui.component.ManagedPanel
import com.proitservices.proidentity.access.ui.component.TotpModal
import com.proitservices.proidentity.access.ui.component.TunnelDetailPanel
import com.proitservices.proidentity.access.ui.component.TunnelListPanel
import com.proitservices.proidentity.access.ui.viewmodel.ManagedViewModel
import com.proitservices.proidentity.access.ui.viewmodel.TunnelViewModel

@Composable
fun MainScreen(
    tunnelVm: TunnelViewModel,
    managedVm: ManagedViewModel,
    onPickFile: () -> Unit,
    prefillConfig: String?,
    onPrefillConsumed: () -> Unit
) {
    val tunnelState by tunnelVm.uiState.collectAsState()
    val managedState by managedVm.uiState.collectAsState()

    var showImport by remember { mutableStateOf(false) }
    var importConfig by remember { mutableStateOf("") }
    var importError by remember { mutableStateOf<String?>(null) }
    var importLoading by remember { mutableStateOf(false) }

    LaunchedEffect(prefillConfig) {
        if (!prefillConfig.isNullOrEmpty()) {
            importConfig = prefillConfig
            showImport = true
            onPrefillConsumed()
        }
    }

    val snackbarState = remember { SnackbarHostState() }
    LaunchedEffect(tunnelState.error) {
        tunnelState.error?.let { snackbarState.showSnackbar(it); tunnelVm.clearError() }
    }
    LaunchedEffect(managedState.error) {
        managedState.error?.let { snackbarState.showSnackbar(it); managedVm.clearError() }
    }

    val managedPanelContent: @Composable () -> Unit = {
        ManagedPanel(
            settings = managedState.settings,
            serverStatuses = managedState.serverStatuses,
            isLoading = managedState.isLoading,
            error = managedState.error,
            onLoginClick = { managedVm.openLoginModal() },
            onLogoutClick = { managedVm.logout() },
            onRefreshClick = { managedVm.refreshServers() },
            onConnectServer = { managedVm.connectServer(it) },
            onDisconnectServer = { managedVm.disconnectServer(it) }
        )
    }

    Scaffold(snackbarHost = { SnackbarHost(snackbarState) }) { _ ->
        BoxWithConstraints(Modifier.fillMaxSize()) {
            val isWide = maxWidth >= 600.dp
            val selectedTunnel = tunnelState.selectedTunnelId?.let { id ->
                tunnelState.tunnels.find { it.id == id }
            }

            if (isWide) {
                Row(Modifier.fillMaxSize()) {
                    Box(Modifier.width(280.dp)) {
                        TunnelListPanel(
                            tunnels = tunnelState.tunnels,
                            selectedId = tunnelState.selectedTunnelId,
                            onSelect = { tunnelVm.selectTunnel(it) },
                            onImportClick = { showImport = true },
                            managedSection = managedPanelContent
                        )
                    }
                    Box(Modifier.weight(1f).fillMaxHeight()) {
                        if (selectedTunnel != null) {
                            TunnelDetailPanel(
                                tunnel = selectedTunnel,
                                stats = tunnelState.stats[selectedTunnel.id],
                                deleteCountdown = tunnelState.deleteCountdown,
                                onBack = { tunnelVm.clearSelection() },
                                onConnect = { tunnelVm.connectTunnel(selectedTunnel.id) },
                                onDisconnect = { tunnelVm.disconnectTunnel(selectedTunnel.id) },
                                onDeleteClick = { tunnelVm.startDeleteCountdown(selectedTunnel.id) },
                                onDeleteCancel = { tunnelVm.cancelDelete() }
                            )
                        }
                    }
                }
            } else {
                if (selectedTunnel != null) {
                    BackHandler { tunnelVm.clearSelection() }
                    TunnelDetailPanel(
                        tunnel = selectedTunnel,
                        stats = tunnelState.stats[selectedTunnel.id],
                        deleteCountdown = tunnelState.deleteCountdown,
                        onBack = { tunnelVm.clearSelection() },
                        onConnect = { tunnelVm.connectTunnel(selectedTunnel.id) },
                        onDisconnect = { tunnelVm.disconnectTunnel(selectedTunnel.id) },
                        onDeleteClick = { tunnelVm.startDeleteCountdown(selectedTunnel.id) },
                        onDeleteCancel = { tunnelVm.cancelDelete() }
                    )
                } else {
                    TunnelListPanel(
                        tunnels = tunnelState.tunnels,
                        selectedId = null,
                        onSelect = { tunnelVm.selectTunnel(it) },
                        onImportClick = { showImport = true },
                        managedSection = managedPanelContent
                    )
                }
            }
        }
    }

    // Modals (rendered outside Scaffold to float above everything)
    if (showImport) {
        ImportModal(
            onImport = { name, config ->
                importLoading = true
                importError = null
                tunnelVm.importTunnel(name, config) { ok, err ->
                    importLoading = false
                    if (ok) { showImport = false; importConfig = "" }
                    else importError = err
                }
            },
            onPickFile = onPickFile,
            onDismiss = { showImport = false; importConfig = ""; importError = null },
            isLoading = importLoading,
            error = importError,
            prefillConfig = importConfig.ifEmpty { null }
        )
    }

    if (managedState.showLoginModal) {
        ManagedLoginModal(
            onLogin = { u, p, t -> managedVm.login(u, p, t) },
            onDismiss = { managedVm.dismissLoginModal() },
            isLoading = managedState.isLoading,
            error = managedState.loginError,
            totpEnabled = managedState.settings.totpEnabled
        )
    }

    if (managedState.showTotpModal || managedState.showPushAuth) {
        TotpModal(
            onSubmit = { managedVm.connectServerWithTotp(it) },
            onDismiss = {
                managedVm.dismissTotpModal()
                managedVm.dismissPushAuth()
            },
            pushAuthEnabled = managedState.pushAuthEnabled,
            pushStatus = managedState.pushStatus,
            onSwitchToPush = {
                val serverId = managedState.totpTargetServerId ?: return@TotpModal
                managedVm.connectServer(serverId)
            },
            onSwitchToTotp = {},
            onRetryPush = {
                val serverId = managedState.totpTargetServerId ?: return@TotpModal
                managedVm.connectServer(serverId)
            }
        )
    }
}
