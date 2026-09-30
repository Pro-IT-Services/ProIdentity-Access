package com.proitservices.proidentity.access.ui.model

import androidx.annotation.StringRes
import com.proitservices.proidentity.access.R
import com.proitservices.proidentity.access.model.ServerStatus
import com.proitservices.proidentity.access.model.StatsInfo
import com.proitservices.proidentity.access.model.TunnelInfo
import com.proitservices.proidentity.access.model.TunnelStatus
import com.proitservices.proidentity.access.ui.design.ApertureState
import com.proitservices.proidentity.access.ui.viewmodel.ManagedUiState
import com.proitservices.proidentity.access.ui.viewmodel.TunnelUiState

enum class ConnectionKind(@StringRes val label: Int) {
    Managed(R.string.kind_managed),
    Imported(R.string.kind_imported),
    /** A config the user uploaded to their account, stored encrypted server-side. */
    Cloud(R.string.kind_synced),
}

/**
 * One row on the connection hub. Unifies managed servers, locally imported
 * WireGuard tunnels, and server-stored user configs ("uconf-*").
 */
data class Connection(
    /** Stable key: "server:<id>" for managed servers, "tunnel:<id>" for tunnels. */
    val key: String,
    val name: String,
    val kind: ConnectionKind,
    val status: TunnelStatus,
    /** Subnet (managed) or first interface address (tunnels), shown in mono. */
    val detail: String,
    val serverId: String? = null,
    /** Live tunnel for this connection, if one exists. */
    val tunnelId: String? = null,
    val error: String? = null,
) {
    val isActive get() = status == TunnelStatus.CONNECTED || status == TunnelStatus.CONNECTING
    val canDelete get() = kind != ConnectionKind.Managed
}

fun buildConnections(managed: ManagedUiState, tunnels: TunnelUiState): List<Connection> {
    val rows = mutableListOf<Connection>()
    val tunnelById = tunnels.tunnels.associateBy { it.id }
    val managedTunnelIds = mutableSetOf<String>()

    managed.serverStatuses.values.sortedBy { it.server.name.lowercase() }.forEach { ss ->
        val tunnel = ss.tunnelId?.let { tunnelById[it] }
        ss.tunnelId?.let { managedTunnelIds += it }
        rows += Connection(
            key = "server:${ss.server.id}",
            name = ss.server.name,
            kind = ConnectionKind.Managed,
            status = serverStatus(ss, tunnel),
            detail = tunnel?.addresses?.firstOrNull() ?: ss.server.subnet,
            serverId = ss.server.id,
            tunnelId = ss.tunnelId,
            error = ss.error,
        )
    }

    tunnels.tunnels
        .filter { !it.isManaged && it.id !in managedTunnelIds && !it.id.startsWith("managed-") }
        .filter { it.id != tunnels.deletePendingId }
        .forEach { t ->
            rows += Connection(
                key = "tunnel:${t.id}",
                name = t.name,
                kind = if (t.id.startsWith("uconf-")) ConnectionKind.Cloud else ConnectionKind.Imported,
                status = t.status,
                detail = t.addresses.firstOrNull() ?: "",
                tunnelId = t.id,
                error = t.error,
            )
        }
    return rows
}

private fun serverStatus(ss: ServerStatus, tunnel: TunnelInfo?): TunnelStatus = when {
    ss.connecting -> TunnelStatus.CONNECTING
    ss.connected -> tunnel?.status ?: TunnelStatus.CONNECTED
    ss.error != null -> TunnelStatus.ERROR
    else -> TunnelStatus.DISCONNECTED
}

/** The connection the primary button acts on: the active one, else last used, else first. */
fun primaryConnection(rows: List<Connection>, lastKey: String): Connection? =
    rows.firstOrNull { it.isActive }
        ?: rows.firstOrNull { it.key == lastKey }
        ?: rows.firstOrNull()

fun TunnelStatus.toAperture(): ApertureState = when (this) {
    TunnelStatus.CONNECTED -> ApertureState.Connected
    TunnelStatus.CONNECTING -> ApertureState.Connecting
    TunnelStatus.ERROR -> ApertureState.Error
    TunnelStatus.DISCONNECTED -> ApertureState.Idle
}

/** Live numbers for a connection, looked up by its tunnel. */
data class Live(val stats: StatsInfo?, val connectedAt: Long?)

fun TunnelUiState.liveFor(c: Connection?): Live =
    Live(c?.tunnelId?.let { stats[it] }, c?.tunnelId?.let { connectedAt[it] })
