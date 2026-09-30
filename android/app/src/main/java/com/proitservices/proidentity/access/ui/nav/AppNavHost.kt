package com.proitservices.proidentity.access.ui.nav

import android.net.Uri
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.TouchApp
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SnackbarDuration
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Surface
import androidx.compose.material3.VerticalDivider
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.R
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.proitservices.proidentity.access.bridge.defaultDeviceName
import com.proitservices.proidentity.access.model.TunnelInfo
import com.proitservices.proidentity.access.ui.design.EmptyState
import com.proitservices.proidentity.access.ui.model.Connection
import com.proitservices.proidentity.access.ui.model.ConnectionKind
import com.proitservices.proidentity.access.ui.model.buildConnections
import com.proitservices.proidentity.access.ui.model.liveFor
import com.proitservices.proidentity.access.ui.model.primaryConnection
import com.proitservices.proidentity.access.ui.screen.ConnectionDetailScreen
import com.proitservices.proidentity.access.ui.screen.HomeScreen
import com.proitservices.proidentity.access.ui.screen.LicensesScreen
import com.proitservices.proidentity.access.ui.screen.SettingsScreen
import com.proitservices.proidentity.access.ui.sheet.ConnectAuthSheet
import com.proitservices.proidentity.access.ui.sheet.ImportSheet
import com.proitservices.proidentity.access.ui.viewmodel.ManagedViewModel
import com.proitservices.proidentity.access.ui.viewmodel.TunnelViewModel
import kotlinx.coroutines.launch

/** A configuration picked from a file, waiting to be shown in the import sheet. */
data class ImportPrefill(val name: String?, val config: String)

private const val HOME = "home"
private const val DETAIL = "detail/{key}"
private const val SETTINGS = "settings"
private const val LICENSES = "licenses"

@Composable
fun AppNavHost(
    tunnelVm: TunnelViewModel,
    managedVm: ManagedViewModel,
    pendingImport: ImportPrefill?,
    onImportConsumed: () -> Unit,
    onPickFile: () -> Unit,
    /** Runs the action once VPN permission is granted (asks the system if needed). */
    withVpnPermission: (() -> Unit) -> Unit,
    onSignIn: () -> Unit,
) {
    val tunnelState by tunnelVm.uiState.collectAsStateWithLifecycle()
    val managedState by managedVm.uiState.collectAsStateWithLifecycle()
    val connections = remember(tunnelState, managedState) { buildConnections(managedState, tunnelState) }
    val active = connections.firstOrNull { it.isActive }
    val primary = primaryConnection(connections, tunnelVm.lastConnectionKey())

    val nav = rememberNavController()
    val snackbar = remember { SnackbarHostState() }
    val scope = rememberCoroutineScope()
    val context = LocalContext.current
    val version = remember {
        runCatching { context.packageManager.getPackageInfo(context.packageName, 0).versionName }.getOrNull().orEmpty()
    }

    var showImport by rememberSaveable { mutableStateOf(false) }
    var importLoading by remember { mutableStateOf(false) }
    var importError by remember { mutableStateOf<String?>(null) }

    // ── Actions ──────────────────────────────────────────────────────────
    val connect: (Connection) -> Unit = { c ->
        when {
            c.kind == ConnectionKind.Managed && c.serverId != null ->
                withVpnPermission { managedVm.connectServer(c.serverId) }
            c.tunnelId != null -> tunnelVm.connectTunnel(c.tunnelId) // asks for VPN permission itself
        }
    }
    val disconnect: (Connection) -> Unit = { c ->
        if (c.kind == ConnectionKind.Managed && c.serverId != null) managedVm.disconnectServer(c.serverId)
        else c.tunnelId?.let(tunnelVm::disconnectTunnel)
    }
    val delete: (Connection) -> Unit = { c -> c.tunnelId?.let(tunnelVm::startDeleteCountdown) }
    val refresh = { managedVm.refreshServers(); tunnelVm.refresh() }
    fun tunnelFor(c: Connection): TunnelInfo? = c.tunnelId?.let { id -> tunnelState.tunnels.find { it.id == id } }

    // ── Feedback ─────────────────────────────────────────────────────────
    LaunchedEffect(pendingImport) { if (pendingImport != null) showImport = true }
    LaunchedEffect(tunnelState.error) {
        tunnelState.error?.let { snackbar.showSnackbar(it); tunnelVm.clearError() }
    }
    LaunchedEffect(managedState.error) {
        managedState.error?.let { snackbar.showSnackbar(it); managedVm.clearError() }
    }
    val pendingDelete = tunnelState.deletePendingId
    val pendingName = remember(pendingDelete) { tunnelState.tunnels.find { it.id == pendingDelete }?.name }
    val deletedMessage = if (pendingName != null) stringResource(R.string.snackbar_deleted, pendingName)
                         else stringResource(R.string.snackbar_deleted_configuration)
    val undoLabel = stringResource(R.string.common_undo)
    LaunchedEffect(pendingDelete) {
        // Keyed on the pending id: when the countdown commits, this effect is
        // cancelled and the snackbar dismisses itself.
        if (pendingDelete != null) {
            val result = snackbar.showSnackbar(
                deletedMessage,
                actionLabel = undoLabel,
                duration = SnackbarDuration.Indefinite,
            )
            if (result == SnackbarResult.ActionPerformed) tunnelVm.cancelDelete()
        }
    }

    // ── Screens ──────────────────────────────────────────────────────────
    NavHost(
        navController = nav,
        startDestination = HOME,
        enterTransition = { slideInHorizontally { it / 4 } + fadeIn() },
        exitTransition = { fadeOut() },
        popEnterTransition = { fadeIn() },
        popExitTransition = { slideOutHorizontally { it / 4 } + fadeOut() },
    ) {
        composable(HOME) {
            BoxWithConstraints(Modifier.fillMaxSize()) {
                val wide = maxWidth >= 600.dp
                if (wide) {
                    var selectedKey by rememberSaveable { mutableStateOf<String?>(null) }
                    val selected = connections.find { it.key == selectedKey } ?: active ?: primary
                    Row(Modifier.fillMaxSize()) {
                        HomeScreen(
                            connections = connections, active = active, primary = primary,
                            live = tunnelState.liveFor(active), settings = managedState.settings,
                            refreshing = managedState.isLoading, snackbar = snackbar,
                            onRefresh = refresh, onOpen = { selectedKey = it.key },
                            onConnect = connect, onDisconnect = disconnect,
                            onImport = { showImport = true }, onSettings = { nav.navigate(SETTINGS) },
                            onSignIn = onSignIn, selectedKey = selected?.key, showPrimaryBar = false,
                            modifier = Modifier.weight(0.46f),
                        )
                        VerticalDivider()
                        Box(Modifier.weight(0.54f).fillMaxHeight()) {
                            if (selected != null) {
                                ConnectionDetailScreen(
                                    connection = selected, tunnel = tunnelFor(selected),
                                    live = tunnelState.liveFor(selected), onBack = {},
                                    onConnect = connect, onDisconnect = disconnect,
                                    onDelete = { delete(it); selectedKey = null }, embedded = true,
                                )
                            } else {
                                Surface(color = MaterialTheme.colorScheme.background, modifier = Modifier.fillMaxSize()) {
                                    Box(contentAlignment = Alignment.Center) {
                                        EmptyState(
                                            icon = Icons.Outlined.TouchApp,
                                            title = stringResource(R.string.home_select_connection_title),
                                            body = stringResource(R.string.home_select_connection_body),
                                        )
                                    }
                                }
                            }
                        }
                    }
                } else {
                    HomeScreen(
                        connections = connections, active = active, primary = primary,
                        live = tunnelState.liveFor(active), settings = managedState.settings,
                        refreshing = managedState.isLoading, snackbar = snackbar,
                        onRefresh = refresh,
                        onOpen = { nav.navigate("detail/${Uri.encode(it.key)}") },
                        onConnect = connect, onDisconnect = disconnect,
                        onImport = { showImport = true }, onSettings = { nav.navigate(SETTINGS) },
                        onSignIn = onSignIn,
                    )
                }
            }
        }
        composable(DETAIL) { entry ->
            val key = Uri.decode(entry.arguments?.getString("key").orEmpty())
            val c = connections.find { it.key == key }
            if (c == null) {
                // Deleted, or a managed server vanished after sign-out.
                LaunchedEffect(Unit) { if (nav.currentBackStackEntry?.id == entry.id) nav.popBackStack() }
            } else {
                ConnectionDetailScreen(
                    connection = c, tunnel = tunnelFor(c), live = tunnelState.liveFor(c),
                    onBack = { nav.popBackStack() },
                    onConnect = connect, onDisconnect = disconnect,
                    onDelete = { delete(it); nav.popBackStack() },
                )
            }
        }
        composable(SETTINGS) {
            SettingsScreen(
                settings = managedState.settings,
                deviceName = remember { defaultDeviceName() },
                version = version,
                onBack = { nav.popBackStack() },
                onSignIn = onSignIn,
                onSignOut = managedVm::logout,
                onReset = managedVm::resetApp,
                onLicenses = { nav.navigate(LICENSES) },
            )
        }
        composable(LICENSES) {
            LicensesScreen(onBack = { nav.popBackStack() })
        }
    }

    // ── Sheets ───────────────────────────────────────────────────────────
    if (managedState.showTotpModal || managedState.showPushAuth) {
        val target = managedState.totpTargetServerId
        ConnectAuthSheet(
            serverName = managedState.serverStatuses[target]?.server?.name ?: stringResource(R.string.connect_auth_your_server),
            pushMode = managedState.showPushAuth,
            pushStatus = managedState.pushStatus,
            pushAvailable = managedState.pushAuthEnabled,
            onSubmitCode = managedVm::connectServerWithTotp,
            onRetryPush = managedVm::retryPushConnect,
            onUseCode = managedVm::switchConnectToTotp,
            onDismiss = { managedVm.dismissTotpModal(); managedVm.dismissPushAuth() },
        )
    }
    if (showImport) {
        ImportSheet(
            prefillName = pendingImport?.name,
            prefillConfig = pendingImport?.config,
            loading = importLoading,
            error = importError,
            onPickFile = onPickFile,
            onImport = { name, cfg ->
                importLoading = true
                importError = null
                tunnelVm.importTunnel(name, cfg) { ok, err ->
                    importLoading = false
                    if (ok) {
                        showImport = false
                        onImportConsumed()
                        scope.launch { snackbar.showSnackbar(context.getString(R.string.snackbar_imported, name)) }
                    } else {
                        importError = err ?: context.getString(R.string.import_error_invalid)
                    }
                }
            },
            onDismiss = { showImport = false; importError = null; onImportConsumed() },
        )
    }
}
