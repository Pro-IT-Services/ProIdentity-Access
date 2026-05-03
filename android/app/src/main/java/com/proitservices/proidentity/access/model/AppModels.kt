package com.proitservices.proidentity.access.model

import com.proitservices.proidentity.access.bridge.ServerInfo

enum class TunnelStatus { CONNECTED, CONNECTING, DISCONNECTED, ERROR }

data class PeerInfo(
    val publicKey: String,
    val endpoint: String?,
    val allowedIps: List<String>,
    val persistentKeepalive: Int?
)

data class TunnelInfo(
    val id: String,
    val name: String,
    val status: TunnelStatus,
    val addresses: List<String>,
    val dns: List<String>,
    val mtu: Int?,
    val listenPort: Int?,
    val privateKey: String,
    val peers: List<PeerInfo>,
    val isManaged: Boolean,
    val error: String?
)

data class StatsInfo(
    val tunnelId: String,
    val rxBytes: Long,
    val txBytes: Long,
    val lastHandshakeMillis: Long
)

data class ManagedSettings(
    val serverUrl: String = "",
    val username: String = "",
    val isAdmin: Boolean = false,
    val loggedIn: Boolean = false,
    val vpnName: String = "",
    val totpEnabled: Boolean = false
)

data class ServerStatus(
    val server: ServerInfo,
    val connected: Boolean = false,
    val tunnelId: String? = null,
    val connecting: Boolean = false,
    val error: String? = null
)
