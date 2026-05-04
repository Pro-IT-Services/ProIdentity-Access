package com.proitservices.proidentity.access.ui.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Divider
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.model.PeerInfo
import com.proitservices.proidentity.access.model.StatsInfo
import com.proitservices.proidentity.access.model.TunnelInfo
import com.proitservices.proidentity.access.model.TunnelStatus

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TunnelDetailPanel(
    tunnel: TunnelInfo,
    stats: StatsInfo?,
    deleteCountdown: Int?,
    onBack: () -> Unit,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit,
    onDeleteClick: () -> Unit,
    onDeleteCancel: () -> Unit
) {
    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text(tunnel.name, fontWeight = FontWeight.SemiBold, maxLines = 1) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            StatusCard(tunnel = tunnel, onConnect = onConnect, onDisconnect = onDisconnect)

            if (tunnel.status == TunnelStatus.CONNECTED && stats != null) {
                StatsCard(stats = stats)
            }

            if (!tunnel.isManaged && tunnel.addresses.isNotEmpty()) {
                InterfaceCard(tunnel = tunnel)
            }

            if (tunnel.peers.isNotEmpty()) {
                PeersCard(peers = tunnel.peers, isManaged = tunnel.isManaged)
            }

            if (!tunnel.isManaged) {
                DeleteButton(
                    countdown = deleteCountdown,
                    onClick = onDeleteClick,
                    onCancel = onDeleteCancel
                )
            }
        }
    }
}

@Composable
private fun StatusCard(tunnel: TunnelInfo, onConnect: () -> Unit, onDisconnect: () -> Unit) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        modifier = Modifier.fillMaxWidth()
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            StatusDot(status = tunnel.status, modifier = Modifier.size(12.dp))
            Spacer(Modifier.width(10.dp))
            Text(
                text = when (tunnel.status) {
                    TunnelStatus.CONNECTED -> "Connected"
                    TunnelStatus.CONNECTING -> "Connecting..."
                    TunnelStatus.ERROR -> tunnel.error ?: "Error"
                    TunnelStatus.DISCONNECTED -> "Disconnected"
                },
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.weight(1f)
            )
            val isConnecting = tunnel.status == TunnelStatus.CONNECTING
            val isConnected = tunnel.status == TunnelStatus.CONNECTED
            if (isConnected) {
                Button(
                    onClick = onDisconnect,
                    enabled = !isConnecting,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = MaterialTheme.colorScheme.error
                    )
                ) { Text("Disconnect") }
            } else {
                Button(onClick = onConnect, enabled = !isConnecting) { Text("Connect") }
            }
        }
    }
}

@Composable
private fun StatsCard(stats: StatsInfo) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(Modifier.padding(16.dp)) {
            Text("Statistics", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(12.dp))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                StatItem("Download", formatBytes(stats.rxBytes))
                StatItem("Upload", formatBytes(stats.txBytes))
                StatItem("Handshake", formatHandshake(stats.lastHandshakeMillis))
            }
        }
    }
}

@Composable
private fun StatItem(label: String, value: String) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Bold)
        Text(label, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun InterfaceCard(tunnel: TunnelInfo) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(Modifier.padding(16.dp)) {
            Text("Interface", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(8.dp))
            if (tunnel.addresses.isNotEmpty()) InfoRow("Addresses", tunnel.addresses.joinToString(", "))
            if (tunnel.dns.isNotEmpty()) InfoRow("DNS", tunnel.dns.joinToString(", "))
            tunnel.mtu?.let { InfoRow("MTU", it.toString()) }
            tunnel.listenPort?.let { InfoRow("Listen Port", it.toString()) }
        }
    }
}

@Composable
private fun PeersCard(peers: List<PeerInfo>, isManaged: Boolean) {
    var expanded by remember { mutableStateOf(true) }
    val clipboard = LocalClipboardManager.current

    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        modifier = Modifier.fillMaxWidth()
    ) {
        Column(Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("Peers (${peers.size})", style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                IconButton(onClick = { expanded = !expanded }, modifier = Modifier.size(24.dp)) {
                    Icon(if (expanded) Icons.Default.ExpandLess else Icons.Default.ExpandMore,
                        contentDescription = if (expanded) "Collapse" else "Expand")
                }
            }
            if (expanded) {
                peers.forEachIndexed { i, peer ->
                    if (i > 0) Divider(modifier = Modifier.padding(vertical = 8.dp),
                        color = MaterialTheme.colorScheme.background, thickness = 0.5.dp)
                    else Spacer(Modifier.height(8.dp))

                    if (!isManaged) {
                        peer.endpoint?.let { InfoRow("Endpoint", it) }
                        peer.persistentKeepalive?.let { InfoRow("Keepalive", "${it}s") }
                    }
                    if (peer.allowedIps.isNotEmpty()) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("Allowed IPs", style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.weight(1f))
                            Text(peer.allowedIps.joinToString(", "),
                                style = MaterialTheme.typography.bodySmall)
                            IconButton(onClick = { clipboard.setText(AnnotatedString(peer.allowedIps.joinToString(","))) },
                                modifier = Modifier.size(32.dp)) {
                                Icon(Icons.Default.ContentCopy, contentDescription = "Copy",
                                    modifier = Modifier.size(16.dp))
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(label, style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodySmall)
    }
}

@Composable
private fun DeleteButton(countdown: Int?, onClick: () -> Unit, onCancel: () -> Unit) {
    if (countdown == null) {
        OutlinedButton(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth(),
            colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)
        ) {
            Icon(Icons.Default.Delete, contentDescription = null, modifier = Modifier.size(18.dp))
            Spacer(Modifier.width(8.dp))
            Text("Delete Tunnel")
        }
    } else {
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(
                onClick = onCancel,
                modifier = Modifier.weight(1f),
                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) { Text("Cancel") }
            Button(
                onClick = { },
                modifier = Modifier.weight(1f),
                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error)
            ) { Text("Deleting in ${countdown}s") }
        }
    }
}

private fun formatBytes(bytes: Long): String {
    if (bytes < 1024) return "${bytes} B"
    val kb = bytes / 1024.0
    if (kb < 1024) return "%.1f KB".format(kb)
    val mb = kb / 1024.0
    if (mb < 1024) return "%.1f MB".format(mb)
    return "%.2f GB".format(mb / 1024.0)
}

private fun formatHandshake(millis: Long): String {
    if (millis == 0L) return "Never"
    val diff = System.currentTimeMillis() - millis
    val seconds = diff / 1000
    if (seconds < 60) return "${seconds}s ago"
    if (seconds < 3600) return "${seconds / 60}m ago"
    return "${seconds / 3600}h ago"
}
