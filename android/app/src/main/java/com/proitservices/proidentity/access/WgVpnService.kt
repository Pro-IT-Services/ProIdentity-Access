package com.proitservices.proidentity.access

import android.net.VpnService

class WgVpnService : VpnService() {
    companion object {
        var instance: WgVpnService? = null
        val stateListeners = mutableListOf<(String, String) -> Unit>() // (tunnelId, state)
    }

    data class TunnelEntry(
        val id: String,
        val name: String,
        val config: String,  // raw WireGuard config text
        var state: String = "disconnected"  // "connected", "disconnected", "error"
    )

    private var backend: com.wireguard.android.backend.GoBackend? = null
    private val activeTunnels = mutableMapOf<String, com.wireguard.android.backend.Tunnel>()
    private val tunnelEntries = mutableMapOf<String, TunnelEntry>()  // id â†’ entry

    override fun onCreate() {
        super.onCreate()
        instance = this
        backend = com.wireguard.android.backend.GoBackend(this)
    }

    override fun onDestroy() {
        instance = null
        super.onDestroy()
    }

    // Returns tunnel entry. Throws if not found.
    fun importTunnel(id: String, name: String, config: String): TunnelEntry {
        val entry = TunnelEntry(id, name, config)
        tunnelEntries[id] = entry
        return entry
    }

    fun connectTunnel(id: String) {
        val entry = tunnelEntries[id] ?: throw IllegalArgumentException("Tunnel not found: $id")
        val parsed = com.wireguard.config.Config.parse(java.io.BufferedReader(java.io.StringReader(entry.config)))
        val tunnel = object : com.wireguard.android.backend.Tunnel {
            override fun getName() = entry.name
            override fun onStateChange(newState: com.wireguard.android.backend.Tunnel.State) {
                entry.state = if (newState == com.wireguard.android.backend.Tunnel.State.UP) "connected" else "disconnected"
                stateListeners.forEach { it(id, entry.state) }
            }
        }
        activeTunnels[id] = tunnel
        backend?.setState(tunnel, com.wireguard.android.backend.Tunnel.State.UP, parsed)
        entry.state = "connected"
        stateListeners.forEach { it(id, "connected") }
    }

    fun disconnectTunnel(id: String) {
        val tunnel = activeTunnels[id] ?: return
        backend?.setState(tunnel, com.wireguard.android.backend.Tunnel.State.DOWN, null)
        activeTunnels.remove(id)
        tunnelEntries[id]?.state = "disconnected"
        stateListeners.forEach { it(id, "disconnected") }
    }

    fun deleteTunnel(id: String) {
        disconnectTunnel(id)
        tunnelEntries.remove(id)
    }

    fun clearAllTunnels() {
        tunnelEntries.keys.toList().forEach { deleteTunnel(it) }
    }

    fun listTunnels(): List<TunnelEntry> = tunnelEntries.values.toList()

    fun getStats(id: String): com.wireguard.android.backend.Statistics? {
        val tunnel = activeTunnels[id] ?: return null
        return backend?.getStatistics(tunnel)
    }
}
