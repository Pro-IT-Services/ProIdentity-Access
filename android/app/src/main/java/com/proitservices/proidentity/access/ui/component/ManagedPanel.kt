package com.proitservices.proidentity.access.ui.component

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material.icons.filled.Cloud
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Divider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.model.ManagedSettings
import com.proitservices.proidentity.access.model.ServerStatus

@Composable
fun ManagedPanel(
    settings: ManagedSettings,
    serverStatuses: Map<String, ServerStatus>,
    isLoading: Boolean,
    error: String?,
    onLoginClick: () -> Unit,
    onLogoutClick: () -> Unit,
    onRefreshClick: () -> Unit,
    onConnectServer: (String) -> Unit,
    onDisconnectServer: (String) -> Unit
) {
    if (settings.serverUrl.isEmpty()) return

    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
        modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 8.dp)
    ) {
        Column(Modifier.padding(12.dp)) {
            // Header
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Default.Cloud, contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(18.dp))
                Spacer(Modifier.width(8.dp))
                Text(
                    settings.vpnName.ifEmpty { settings.serverUrl },
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.weight(1f)
                )
                if (settings.loggedIn) {
                    IconButton(onClick = onRefreshClick, modifier = Modifier.size(32.dp)) {
                        if (isLoading) CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                        else Icon(Icons.Default.Refresh, contentDescription = "Refresh",
                            modifier = Modifier.size(18.dp))
                    }
                }
            }

            error?.let {
                Spacer(Modifier.height(6.dp))
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }

            Spacer(Modifier.height(10.dp))
            Divider(color = MaterialTheme.colorScheme.background, thickness = 0.5.dp)
            Spacer(Modifier.height(10.dp))

            if (!settings.loggedIn) {
                Button(onClick = onLoginClick, modifier = Modifier.fillMaxWidth()) {
                    Text("Sign In")
                }
            } else {
                // User info
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(settings.username,
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        modifier = Modifier.weight(1f))
                    if (settings.isAdmin) {
                        Icon(Icons.Default.AdminPanelSettings, contentDescription = "Admin",
                            tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(18.dp))
                    }
                    Spacer(Modifier.width(4.dp))
                    IconButton(onClick = onLogoutClick, modifier = Modifier.size(32.dp)) {
                        Icon(Icons.Default.Logout, contentDescription = "Logout",
                            modifier = Modifier.size(18.dp))
                    }
                }

                if (serverStatuses.isNotEmpty()) {
                    Spacer(Modifier.height(10.dp))
                    Text("Servers",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Spacer(Modifier.height(6.dp))
                    serverStatuses.values.forEach { status ->
                        ServerRow(
                            status = status,
                            onConnect = { onConnectServer(status.server.id) },
                            onDisconnect = { onDisconnectServer(status.server.id) }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun ServerRow(
    status: ServerStatus,
    onConnect: () -> Unit,
    onDisconnect: () -> Unit
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        StatusDot(
            status = when {
                status.connecting -> com.proitservices.proidentity.access.model.TunnelStatus.CONNECTING
                status.connected  -> com.proitservices.proidentity.access.model.TunnelStatus.CONNECTED
                else              -> com.proitservices.proidentity.access.model.TunnelStatus.DISCONNECTED
            },
            modifier = Modifier.size(8.dp)
        )
        Spacer(Modifier.width(8.dp))
        Column(Modifier.weight(1f)) {
            Text(status.server.name, style = MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Medium)
            status.error?.let {
                Text(it, style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.error)
            }
        }
        Spacer(Modifier.width(8.dp))
        if (status.connected) {
            OutlinedButton(
                onClick = onDisconnect,
                modifier = Modifier.height(32.dp),
                colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)
            ) { Text("Disconnect", style = MaterialTheme.typography.labelSmall) }
        } else {
            Button(
                onClick = onConnect,
                modifier = Modifier.height(32.dp),
                enabled = !status.connecting
            ) {
                if (status.connecting) CircularProgressIndicator(Modifier.size(14.dp), strokeWidth = 2.dp)
                else Text("Connect", style = MaterialTheme.typography.labelSmall)
            }
        }
    }
}
