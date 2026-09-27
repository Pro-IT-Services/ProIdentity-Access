package com.proitservices.proidentity.access.ui.screen

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.DeleteOutline
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.model.TunnelInfo
import com.proitservices.proidentity.access.model.TunnelStatus
import com.proitservices.proidentity.access.ui.design.InfoCard
import com.proitservices.proidentity.access.ui.design.InfoRow
import com.proitservices.proidentity.access.ui.design.NoticeCard
import com.proitservices.proidentity.access.ui.design.PrimaryActionBar
import com.proitservices.proidentity.access.ui.design.SectionLabel
import com.proitservices.proidentity.access.ui.design.StatusAperture
import com.proitservices.proidentity.access.ui.design.formatAgo
import com.proitservices.proidentity.access.ui.design.formatBytes
import com.proitservices.proidentity.access.ui.design.formatDuration
import com.proitservices.proidentity.access.ui.design.statusColor
import com.proitservices.proidentity.access.ui.design.statusText
import com.proitservices.proidentity.access.ui.model.Connection
import com.proitservices.proidentity.access.ui.model.ConnectionKind
import com.proitservices.proidentity.access.ui.model.Live
import com.proitservices.proidentity.access.ui.model.toAperture
import com.proitservices.proidentity.access.ui.theme.Brand
import kotlinx.coroutines.delay

/**
 * Details for one connection. Shows addressing and live traffic — never keys.
 * [embedded] hides the back button when shown in the tablet detail pane.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConnectionDetailScreen(
    connection: Connection,
    tunnel: TunnelInfo?,
    live: Live,
    onBack: () -> Unit,
    onConnect: (Connection) -> Unit,
    onDisconnect: (Connection) -> Unit,
    onDelete: (Connection) -> Unit,
    embedded: Boolean = false,
) {
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(connection.name, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                navigationIcon = {
                    if (!embedded) IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "Back")
                    }
                },
                actions = {
                    if (connection.canDelete) IconButton(onClick = { onDelete(connection) }) {
                        Icon(Icons.Outlined.DeleteOutline, contentDescription = "Delete ${connection.name}")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
        bottomBar = { PrimaryActionBar(connection, onConnect, onDisconnect) },
        containerColor = MaterialTheme.colorScheme.background,
    ) { padding ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp, vertical = 8.dp),
        ) {
            StatusHeader(connection, live)
            Spacer(Modifier.height(16.dp))

            connection.error?.takeIf { connection.status == TunnelStatus.ERROR }?.let {
                NoticeCard(title = "Couldn't connect", body = it, container = Brand.status.warningContainer)
                Spacer(Modifier.height(16.dp))
            }

            SectionLabel("Connection")
            InfoCard {
                InfoRow("Type", when (connection.kind) {
                    ConnectionKind.Managed -> "Managed server"
                    ConnectionKind.Imported -> "Imported on this device"
                    ConnectionKind.Cloud -> "Synced to your account"
                })
                if (connection.kind == ConnectionKind.Managed) InfoRow("Network", connection.detail, mono = true)
                tunnel?.let { t ->
                    if (t.addresses.isNotEmpty()) InfoRow("Address", t.addresses.joinToString("\n"), mono = true)
                    if (t.dns.isNotEmpty()) InfoRow("DNS", t.dns.joinToString("\n"), mono = true)
                    t.peers.mapNotNull { it.endpoint }.distinct().takeIf { it.isNotEmpty() }?.let {
                        InfoRow("Endpoint", it.joinToString("\n"), mono = true)
                    }
                    t.peers.flatMap { it.allowedIps }.distinct().takeIf { it.isNotEmpty() }?.let {
                        InfoRow("Routes", it.joinToString("\n"), mono = true)
                    }
                    t.mtu?.let { InfoRow("MTU", it.toString(), mono = true) }
                }
            }

            if (connection.status == TunnelStatus.CONNECTED) {
                Spacer(Modifier.height(16.dp))
                SectionLabel("Traffic")
                InfoCard {
                    InfoRow("Received", formatBytes(live.stats?.rxBytes ?: 0), mono = true)
                    InfoRow("Sent", formatBytes(live.stats?.txBytes ?: 0), mono = true)
                    InfoRow("Last handshake", formatAgo(live.stats?.lastHandshakeMillis ?: 0))
                }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}

@Composable
private fun StatusHeader(connection: Connection, live: Live) {
    val since = live.connectedAt
    val now by produceState(System.currentTimeMillis(), since) {
        while (since != null) { value = System.currentTimeMillis(); delay(1000) }
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        StatusAperture(connection.status.toAperture(), size = 88.dp)
        Spacer(Modifier.width(16.dp))
        Column {
            Text(statusText(connection.status), style = MaterialTheme.typography.titleLarge, color = statusColor(connection.status))
            Text(
                if (since != null && connection.status == TunnelStatus.CONNECTED) "for ${formatDuration(now - since)}"
                else connection.kind.label,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
