package com.proitservices.proidentity.access.ui.design

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.CloudSync
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.PowerSettingsNew
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.R
import com.proitservices.proidentity.access.model.TunnelStatus
import com.proitservices.proidentity.access.ui.model.Connection
import com.proitservices.proidentity.access.ui.model.ConnectionKind
import com.proitservices.proidentity.access.ui.theme.Brand
import com.proitservices.proidentity.access.ui.theme.MonoStyle
import java.util.Locale

// ── Formatting ────────────────────────────────────────────────────────────

fun formatBytes(bytes: Long): String {
    if (bytes < 1024) return "$bytes B"
    val units = listOf("KB", "MB", "GB", "TB")
    var value = bytes / 1024.0
    var i = 0
    while (value >= 1024 && i < units.lastIndex) { value /= 1024; i++ }
    return String.format(Locale.US, if (value < 10) "%.1f %s" else "%.0f %s", value, units[i])
}

fun formatDuration(millis: Long): String {
    val total = (millis / 1000).coerceAtLeast(0)
    val h = total / 3600
    val m = (total % 3600) / 60
    val s = total % 60
    return if (h > 0) String.format(Locale.US, "%d:%02d:%02d", h, m, s)
    else String.format(Locale.US, "%02d:%02d", m, s)
}

@Composable
fun formatAgo(epochMillis: Long, now: Long = System.currentTimeMillis()): String {
    if (epochMillis <= 0) return stringResource(R.string.time_never)
    val secs = ((now - epochMillis) / 1000).coerceAtLeast(0)
    return when {
        secs < 5 -> stringResource(R.string.time_just_now)
        secs < 60 -> stringResource(R.string.time_seconds_ago, secs)
        secs < 3600 -> stringResource(R.string.time_minutes_ago, secs / 60)
        else -> stringResource(R.string.time_hours_ago, secs / 3600)
    }
}

// ── Status ────────────────────────────────────────────────────────────────

@Composable
fun statusText(status: TunnelStatus): String = when (status) {
    TunnelStatus.CONNECTED -> stringResource(R.string.status_connected)
    TunnelStatus.CONNECTING -> stringResource(R.string.status_connecting)
    TunnelStatus.ERROR -> stringResource(R.string.status_error)
    TunnelStatus.DISCONNECTED -> stringResource(R.string.status_available)
}

@Composable
fun statusColor(status: TunnelStatus): Color = when (status) {
    TunnelStatus.CONNECTED -> Brand.status.connected
    TunnelStatus.CONNECTING -> MaterialTheme.colorScheme.primary
    TunnelStatus.ERROR -> Brand.status.warning
    TunnelStatus.DISCONNECTED -> MaterialTheme.colorScheme.onSurfaceVariant
}

@Composable
fun StatusDot(status: TunnelStatus, modifier: Modifier = Modifier) {
    Box(modifier.size(8.dp).clip(CircleShape).background(statusColor(status)))
}

private fun ConnectionKind.icon(): ImageVector = when (this) {
    ConnectionKind.Managed -> Icons.Outlined.Dns
    ConnectionKind.Imported -> Icons.Outlined.Description
    ConnectionKind.Cloud -> Icons.Outlined.CloudSync
}

// ── Building blocks ───────────────────────────────────────────────────────

@Composable
fun SectionLabel(text: String, modifier: Modifier = Modifier) {
    Text(
        text.uppercase(),
        style = MaterialTheme.typography.labelMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = modifier
            .padding(horizontal = 20.dp, vertical = 8.dp)
            .semantics { heading() },
    )
}

/**
 * One connection on the hub. Tap opens details; the trailing 48dp button
 * connects or disconnects in one tap.
 */
@Composable
fun ConnectionRow(
    connection: Connection,
    onOpen: () -> Unit,
    onToggle: () -> Unit,
    modifier: Modifier = Modifier,
    selected: Boolean = false,
) {
    val status = connection.status
    val active = connection.isActive
    val accent = statusColor(status)
    val kindLabel = stringResource(connection.kind.label)
    val stateLabel = statusText(status)
    val toggleDescription = if (active) stringResource(R.string.common_disconnect_named, connection.name)
                            else stringResource(R.string.common_connect_to, connection.name)
    val description = buildString {
        append(connection.name).append(", ").append(kindLabel).append(", ")
        append(stateLabel)
        if (connection.detail.isNotEmpty()) append(", ").append(connection.detail)
    }
    Surface(
        onClick = onOpen,
        shape = MaterialTheme.shapes.medium,
        color = if (selected) MaterialTheme.colorScheme.surfaceContainerHigh
                else MaterialTheme.colorScheme.surfaceContainer,
        modifier = modifier
            .fillMaxWidth()
            .semantics(mergeDescendants = true) {},
    ) {
        Row(
            Modifier.heightIn(min = 72.dp).padding(start = 16.dp, end = 8.dp, top = 12.dp, bottom = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(40.dp)
                    .clip(CircleShape)
                    .background(
                        if (status == TunnelStatus.CONNECTED) Brand.status.connectedContainer
                        else MaterialTheme.colorScheme.surfaceVariant,
                    ),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    connection.kind.icon(), contentDescription = null,
                    tint = if (status == TunnelStatus.CONNECTED) Brand.status.connected
                           else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.size(20.dp),
                )
            }
            Column(
                Modifier
                    .weight(1f)
                    .padding(horizontal = 14.dp)
                    .clearAndSetSemantics { contentDescription = description },
            ) {
                Text(
                    connection.name,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
                Spacer(Modifier.height(2.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    StatusDot(status)
                    Spacer(Modifier.width(6.dp))
                    Text(stateLabel, style = MaterialTheme.typography.bodySmall, color = accent)
                }
                Text(
                    listOf(kindLabel, connection.detail).filter { it.isNotEmpty() }.joinToString("  ·  "),
                    style = MonoStyle,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1, overflow = TextOverflow.Ellipsis,
                )
            }
            Box(Modifier.size(48.dp), contentAlignment = Alignment.Center) {
                if (status == TunnelStatus.CONNECTING) {
                    CircularProgressIndicator(Modifier.size(22.dp), strokeWidth = 2.5.dp)
                } else {
                    IconButton(
                        onClick = onToggle,
                        modifier = Modifier.semantics {
                            role = Role.Button
                            contentDescription = toggleDescription
                        },
                    ) {
                        Icon(
                            Icons.Outlined.PowerSettingsNew, contentDescription = null,
                            tint = if (active) Brand.status.disconnect else MaterialTheme.colorScheme.primary,
                        )
                    }
                }
            }
        }
    }
}

/** Bottom thumb-zone action. Pinned above the navigation bar. */
@Composable
fun PrimaryActionBar(
    target: Connection?,
    onConnect: (Connection) -> Unit,
    onDisconnect: (Connection) -> Unit,
    modifier: Modifier = Modifier,
) {
    if (target == null) return
    Surface(color = MaterialTheme.colorScheme.surface, modifier = modifier.fillMaxWidth()) {
        Box(Modifier.navigationBarsPadding().padding(horizontal = 20.dp, vertical = 12.dp)) {
            val shape = MaterialTheme.shapes.large
            when (target.status) {
                TunnelStatus.CONNECTED -> Button(
                    onClick = { onDisconnect(target) },
                    shape = shape,
                    colors = ButtonDefaults.buttonColors(
                        containerColor = Brand.status.disconnect,
                        contentColor = Brand.status.onDisconnect,
                    ),
                    modifier = Modifier.fillMaxWidth().height(56.dp),
                ) {
                    Icon(Icons.Outlined.PowerSettingsNew, contentDescription = null)
                    Spacer(Modifier.width(10.dp))
                    Text(stringResource(R.string.common_disconnect), style = MaterialTheme.typography.titleMedium)
                }
                TunnelStatus.CONNECTING -> OutlinedButton(
                    onClick = { onDisconnect(target) },
                    shape = shape,
                    modifier = Modifier.fillMaxWidth().height(56.dp),
                ) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                    Spacer(Modifier.width(10.dp))
                    Text(stringResource(R.string.common_cancel), style = MaterialTheme.typography.titleMedium)
                }
                else -> Button(
                    onClick = { onConnect(target) },
                    shape = shape,
                    modifier = Modifier.fillMaxWidth().height(56.dp),
                ) {
                    Icon(Icons.Outlined.PowerSettingsNew, contentDescription = null)
                    Spacer(Modifier.width(10.dp))
                    Text(
                        stringResource(R.string.common_connect_to, target.name),
                        style = MaterialTheme.typography.titleMedium,
                        maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
            }
        }
    }
}

@Composable
fun EmptyState(
    icon: ImageVector,
    title: String,
    body: String,
    modifier: Modifier = Modifier,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
) {
    Column(
        modifier.fillMaxWidth().padding(horizontal = 32.dp, vertical = 40.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Box(
            Modifier.size(64.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center,
        ) {
            Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(28.dp))
        }
        Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        Text(
            body, style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center,
        )
        if (actionLabel != null && onAction != null) {
            Spacer(Modifier.height(4.dp))
            Button(onClick = onAction, shape = MaterialTheme.shapes.large) { Text(actionLabel) }
        }
    }
}

/** A full-width notice with an optional action, e.g. "You're signed out". */
@Composable
fun NoticeCard(
    title: String,
    body: String,
    modifier: Modifier = Modifier,
    container: Color = MaterialTheme.colorScheme.surfaceContainerHigh,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
) {
    Surface(color = container, shape = MaterialTheme.shapes.medium, modifier = modifier.fillMaxWidth()) {
        Row(Modifier.padding(start = 16.dp, top = 12.dp, bottom = 12.dp, end = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(title, style = MaterialTheme.typography.titleSmall)
                Text(body, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (actionLabel != null && onAction != null) {
                TextButton(onClick = onAction, contentPadding = PaddingValues(horizontal = 12.dp)) { Text(actionLabel) }
            }
        }
    }
}

/** Label / value line for detail and settings screens. */
@Composable
fun InfoRow(label: String, value: String, modifier: Modifier = Modifier, mono: Boolean = false) {
    Row(
        modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .padding(horizontal = 16.dp, vertical = 10.dp)
            .semantics(mergeDescendants = true) {},
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(0.42f))
        Text(
            value.ifEmpty { "—" },
            style = if (mono) MonoStyle else MaterialTheme.typography.bodyMedium,
            textAlign = TextAlign.End,
            modifier = Modifier.weight(0.58f),
        )
    }
}

/** Grouped container for InfoRows. */
@Composable
fun InfoCard(modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Surface(color = MaterialTheme.colorScheme.surfaceContainer, shape = MaterialTheme.shapes.medium, modifier = modifier.fillMaxWidth()) {
        Column(Modifier.padding(vertical = 4.dp)) { content() }
    }
}

/** App bar title: brand mark + product name. */
@Composable
fun BrandTitle(modifier: Modifier = Modifier) {
    Row(modifier.semantics(mergeDescendants = true) { contentDescription = "ProIdentity Access" }, verticalAlignment = Alignment.CenterVertically) {
        Icon(ApertureMark, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(26.dp))
        Spacer(Modifier.width(10.dp))
        Text("ProIdentity", style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.width(6.dp))
        Text("Access", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
