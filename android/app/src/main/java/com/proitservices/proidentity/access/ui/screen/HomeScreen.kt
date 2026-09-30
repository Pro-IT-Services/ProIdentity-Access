package com.proitservices.proidentity.access.ui.screen

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Add
import androidx.compose.material.icons.outlined.ArrowDownward
import androidx.compose.material.icons.outlined.ArrowUpward
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.R
import com.proitservices.proidentity.access.model.ManagedSettings
import com.proitservices.proidentity.access.model.TunnelStatus
import com.proitservices.proidentity.access.ui.design.BrandTitle
import com.proitservices.proidentity.access.ui.design.ConnectionRow
import com.proitservices.proidentity.access.ui.design.EmptyState
import com.proitservices.proidentity.access.ui.design.NoticeCard
import com.proitservices.proidentity.access.ui.design.PrimaryActionBar
import com.proitservices.proidentity.access.ui.design.SectionLabel
import com.proitservices.proidentity.access.ui.design.StatusAperture
import com.proitservices.proidentity.access.ui.design.formatBytes
import com.proitservices.proidentity.access.ui.design.formatDuration
import com.proitservices.proidentity.access.ui.design.statusColor
import com.proitservices.proidentity.access.ui.model.Connection
import com.proitservices.proidentity.access.ui.model.Live
import com.proitservices.proidentity.access.ui.model.toAperture
import com.proitservices.proidentity.access.ui.theme.MonoStyle
import kotlinx.coroutines.delay

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    connections: List<Connection>,
    active: Connection?,
    primary: Connection?,
    live: Live,
    settings: ManagedSettings,
    refreshing: Boolean,
    snackbar: SnackbarHostState,
    onRefresh: () -> Unit,
    onOpen: (Connection) -> Unit,
    onConnect: (Connection) -> Unit,
    onDisconnect: (Connection) -> Unit,
    onImport: () -> Unit,
    onSettings: () -> Unit,
    onSignIn: () -> Unit,
    modifier: Modifier = Modifier,
    selectedKey: String? = null,
    showPrimaryBar: Boolean = true,
) {
    ConnectionHaptics(active?.status)

    val managed = settings.mode == "managed"
    Scaffold(
        modifier = modifier,
        topBar = {
            TopAppBar(
                title = { BrandTitle() },
                actions = {
                    IconButton(onClick = onImport) { Icon(Icons.Outlined.Add, contentDescription = stringResource(R.string.common_import_configuration)) }
                    IconButton(onClick = onSettings) { Icon(Icons.Outlined.Settings, contentDescription = stringResource(R.string.common_settings)) }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
        bottomBar = { if (showPrimaryBar) PrimaryActionBar(primary, onConnect, onDisconnect) },
        snackbarHost = { SnackbarHost(snackbar) },
        containerColor = MaterialTheme.colorScheme.background,
    ) { padding ->
        PullToRefreshBox(
            isRefreshing = refreshing,
            onRefresh = onRefresh,
            modifier = Modifier.fillMaxSize().padding(padding),
        ) {
            LazyColumn(
                Modifier.fillMaxSize(),
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                if (managed && !settings.loggedIn) {
                    item(key = "signedOut") {
                        NoticeCard(
                            title = stringResource(R.string.home_signed_out_title),
                            body = stringResource(R.string.home_signed_out_body),
                            actionLabel = stringResource(R.string.common_sign_in),
                            onAction = onSignIn,
                        )
                    }
                }
                item(key = "hero") { ConnectionHero(active, primary, live) }
                if (connections.isEmpty()) {
                    item(key = "empty") {
                        when {
                            managed && settings.loggedIn -> EmptyState(
                                icon = Icons.Outlined.Dns,
                                title = stringResource(R.string.home_empty_assigned_title),
                                body = stringResource(R.string.home_empty_assigned_body),
                                actionLabel = stringResource(R.string.common_import_configuration), onAction = onImport,
                            )
                            managed -> EmptyState(
                                icon = Icons.Outlined.Dns,
                                title = stringResource(R.string.home_empty_managed_title),
                                body = stringResource(R.string.home_empty_managed_body),
                                actionLabel = stringResource(R.string.common_import_configuration), onAction = onImport,
                            )
                            else -> EmptyState(
                                icon = Icons.Outlined.Description,
                                title = stringResource(R.string.home_empty_standalone_title),
                                body = stringResource(R.string.home_empty_standalone_body),
                                actionLabel = stringResource(R.string.common_import_configuration), onAction = onImport,
                            )
                        }
                    }
                } else {
                    item(key = "label") { SectionLabel(stringResource(R.string.home_connections), Modifier.padding(top = 6.dp)) }
                    items(connections, key = { it.key }) { c ->
                        ConnectionRow(
                            connection = c,
                            onOpen = { onOpen(c) },
                            onToggle = { if (c.isActive) onDisconnect(c) else onConnect(c) },
                            selected = c.key == selectedKey,
                            modifier = Modifier.animateItem(),
                        )
                    }
                }
                item(key = "bottomSpace") { Spacer(Modifier.height(8.dp)) }
            }
        }
    }
}

/** Confirm on connect, reject on failure — only on real transitions. */
@Composable
private fun ConnectionHaptics(status: TunnelStatus?) {
    val haptics = LocalHapticFeedback.current
    val previous = remember { arrayOf<TunnelStatus?>(null) }
    LaunchedEffect(status) {
        val before = previous[0]
        if (before != null && before != status) {
            when (status) {
                TunnelStatus.CONNECTED -> haptics.performHapticFeedback(HapticFeedbackType.Confirm)
                TunnelStatus.ERROR -> haptics.performHapticFeedback(HapticFeedbackType.Reject)
                else -> {}
            }
        }
        previous[0] = status
    }
}

@Composable
fun ConnectionHero(active: Connection?, primary: Connection?, live: Live) {
    val status = active?.status ?: if (primary?.status == TunnelStatus.ERROR) TunnelStatus.ERROR else TunnelStatus.DISCONNECTED
    val headline = when (status) {
        TunnelStatus.CONNECTED -> stringResource(R.string.status_connected)
        TunnelStatus.CONNECTING -> stringResource(R.string.status_connecting)
        TunnelStatus.ERROR -> stringResource(R.string.status_error)
        TunnelStatus.DISCONNECTED -> stringResource(R.string.status_not_connected)
    }
    val subtitle = when {
        active != null -> active.name
        primary != null -> stringResource(R.string.home_ready_to_connect, primary.name)
        else -> stringResource(R.string.home_add_connection)
    }
    Surface(
        color = MaterialTheme.colorScheme.surfaceContainerLow,
        shape = MaterialTheme.shapes.extraLarge,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(
            Modifier.padding(vertical = 28.dp, horizontal = 20.dp).fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            StatusAperture(status.toAperture(), size = 168.dp)
            Spacer(Modifier.height(18.dp))
            Text(
                headline,
                style = MaterialTheme.typography.headlineMedium,
                color = if (status == TunnelStatus.DISCONNECTED) MaterialTheme.colorScheme.onSurface else statusColor(status),
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
            Spacer(Modifier.height(4.dp))
            Text(
                subtitle,
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
            AnimatedVisibility(visible = status == TunnelStatus.CONNECTED) {
                LiveStats(active, live)
            }
        }
    }
}

@Composable
private fun LiveStats(active: Connection?, live: Live) {
    val since = live.connectedAt
    val now by produceState(System.currentTimeMillis(), since) {
        while (since != null) { value = System.currentTimeMillis(); delay(1000) }
    }
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.padding(top = 16.dp)) {
        if (since != null) {
            Text(formatDuration(now - since), style = MonoStyle.copy(fontSize = MaterialTheme.typography.titleMedium.fontSize), color = MaterialTheme.colorScheme.onSurface)
            Spacer(Modifier.height(10.dp))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(20.dp), verticalAlignment = Alignment.CenterVertically) {
            Traffic(Icons.Outlined.ArrowDownward, stringResource(R.string.common_received), live.stats?.rxBytes ?: 0)
            Traffic(Icons.Outlined.ArrowUpward, stringResource(R.string.common_sent), live.stats?.txBytes ?: 0)
        }
        active?.detail?.takeIf { it.isNotEmpty() }?.let {
            Spacer(Modifier.height(10.dp))
            Text(it, style = MonoStyle, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun Traffic(icon: androidx.compose.ui.graphics.vector.ImageVector, label: String, bytes: Long) {
    Row(
        verticalAlignment = Alignment.CenterVertically,
        modifier = Modifier.semantics(mergeDescendants = true) {},
    ) {
        Icon(icon, contentDescription = label, modifier = Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(4.dp))
        Text(formatBytes(bytes), style = MonoStyle, color = MaterialTheme.colorScheme.onSurface)
    }
}
