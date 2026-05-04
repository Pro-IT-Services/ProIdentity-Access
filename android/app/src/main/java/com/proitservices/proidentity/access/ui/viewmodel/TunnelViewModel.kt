package com.proitservices.proidentity.access.ui.viewmodel

import android.app.Application
import android.content.Intent
import android.net.VpnService
import android.util.Base64
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.proitservices.proidentity.access.WgVpnService
import com.proitservices.proidentity.access.bridge.AppSettings
import com.proitservices.proidentity.access.bridge.AuthInvalidException
import com.proitservices.proidentity.access.bridge.DeviceCrypto
import com.proitservices.proidentity.access.bridge.DeviceRevokedException
import com.proitservices.proidentity.access.bridge.ManagedClient
import com.proitservices.proidentity.access.model.PeerInfo
import com.proitservices.proidentity.access.model.StatsInfo
import com.proitservices.proidentity.access.model.TunnelInfo
import com.proitservices.proidentity.access.model.TunnelStatus
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.BufferedReader
import java.io.StringReader

data class TunnelUiState(
    val tunnels: List<TunnelInfo> = emptyList(),
    val selectedTunnelId: String? = null,
    val stats: Map<String, StatsInfo> = emptyMap(),
    val isLoading: Boolean = false,
    val error: String? = null,
    val deleteCountdown: Int? = null,
    val vpnPermissionNeeded: Boolean = false
)

class TunnelViewModel(application: Application) : AndroidViewModel(application) {

    private val settings = AppSettings(application)

    private val _uiState = MutableStateFlow(TunnelUiState())
    val uiState: StateFlow<TunnelUiState> = _uiState.asStateFlow()

    private val _requestVpnPermission = MutableSharedFlow<Intent>()
    val requestVpnPermission = _requestVpnPermission.asSharedFlow()

    // Tunnel ID queued for connect while waiting for VPN permission
    private var pendingConnectTunnelId: String? = null

    private val statsJobs = mutableMapOf<String, Job>()
    private var deleteCountdownJob: Job? = null
    private var deletePendingId: String? = null

    // Managed session reverse-map so disconnect-by-tunnel-id works
    // Set by ManagedViewModel
    var tunnelToServerMap: Map<String, String> = emptyMap()
    var onManagedDisconnectRequest: ((serverID: String) -> Unit)? = null
    var onAuthInvalid: (() -> Unit)? = null

    init {
        WgVpnService.stateListeners.add { tunnelId, state ->
            val status = when (state) {
                "connected"    -> TunnelStatus.CONNECTED
                "connecting"   -> TunnelStatus.CONNECTING
                "error"        -> TunnelStatus.ERROR
                else           -> TunnelStatus.DISCONNECTED
            }
            _uiState.update { s ->
                s.copy(
                    tunnels = s.tunnels.map {
                        if (it.id == tunnelId) it.copy(status = status) else it
                    }
                )
            }
            if (state == "connected") startStatsPolling(tunnelId)
            else stopStatsPolling(tunnelId)
        }
        refresh()
    }

    fun refresh() {
        viewModelScope.launch(Dispatchers.IO) {
            val service = WgVpnService.instance ?: return@launch
            val entries = service.listTunnels()
            val tunnels = entries.map { e -> buildTunnelInfo(e) }

            // In managed mode also fetch user configs
            val managed = settings.mode == "managed" && settings.token.isNotEmpty()
            val allTunnels = if (managed) {
                val existing = entries.map { it.id }.toSet()
                val extras = fetchUserConfigStubs(existing)
                tunnels + extras
            } else tunnels

            _uiState.update { it.copy(tunnels = allTunnels) }
        }
    }

    private fun buildTunnelInfo(entry: WgVpnService.TunnelEntry): TunnelInfo {
        val (addresses, dns, mtu, listenPort, privateKey, peers) = parseConfig(entry.config)
        val status = when (entry.state) {
            "connected"  -> TunnelStatus.CONNECTED
            "connecting" -> TunnelStatus.CONNECTING
            "error"      -> TunnelStatus.ERROR
            else         -> TunnelStatus.DISCONNECTED
        }
        return TunnelInfo(
            id = entry.id,
            name = entry.name,
            status = status,
            addresses = addresses,
            dns = dns,
            mtu = mtu,
            listenPort = listenPort,
            privateKey = privateKey,
            peers = peers,
            isManaged = tunnelToServerMap.containsKey(entry.id),
            error = null
        )
    }

    private fun fetchUserConfigStubs(existingIds: Set<String>): List<TunnelInfo> {
        return try {
            val client = buildAuthClient()
            client.listUserConfigs().mapNotNull { cfg ->
                val id = "uconf-${cfg.id}"
                if (id in existingIds) null
                else TunnelInfo(
                    id = id, name = cfg.name,
                    status = TunnelStatus.DISCONNECTED,
                    addresses = emptyList(), dns = emptyList(),
                    mtu = null, listenPort = null, privateKey = "",
                    peers = emptyList(), isManaged = false, error = null
                )
            }
        } catch (e: DeviceRevokedException) {
            triggerAuthReset()
            emptyList()
        } catch (e: AuthInvalidException) {
            triggerAuthReset()
            emptyList()
        } catch (_: Exception) { emptyList() }
    }

    fun selectTunnel(id: String) {
        _uiState.update { it.copy(selectedTunnelId = id) }
    }

    fun clearSelection() {
        _uiState.update { it.copy(selectedTunnelId = null) }
    }

    fun connectTunnel(id: String) {
        val ctx = getApplication<Application>()
        val intent = VpnService.prepare(ctx)
        if (intent != null) {
            pendingConnectTunnelId = id
            viewModelScope.launch { _requestVpnPermission.emit(intent) }
            return
        }
        doConnect(id)
    }

    fun onVpnPermissionResult(granted: Boolean) {
        val id = pendingConnectTunnelId ?: return
        pendingConnectTunnelId = null
        if (granted) doConnect(id) else setError("VPN permission denied")
    }

    private fun doConnect(id: String) {
        viewModelScope.launch(Dispatchers.IO) {
            setStatus(id, TunnelStatus.CONNECTING)
            try {
                if (id.startsWith("uconf-")) connectUserConfig(id)
                else WgVpnService.instance?.connectTunnel(id)
            } catch (e: DeviceRevokedException) {
                triggerAuthReset()
            } catch (e: AuthInvalidException) {
                triggerAuthReset()
            } catch (e: Exception) {
                setStatus(id, TunnelStatus.ERROR)
                setError(e.message ?: "Connect failed")
            }
        }
    }

    private fun connectUserConfig(id: String) {
        val configId = id.removePrefix("uconf-")
        val client = buildAuthClient()
        val encBytes = client.downloadUserConfig(configId)
        val rawKey = client.getUserConfigKey()
        val aad = settings.deviceID.toByteArray(Charsets.UTF_8)
        val encStr = String(encBytes, Charsets.UTF_8)
        val decryptedEnvelope = String(
            Base64.decode(encStr, Base64.NO_WRAP),
            Charsets.UTF_8
        )
        val config = String(DeviceCrypto.decryptBody(rawKey, decryptedEnvelope, aad), Charsets.UTF_8)
        val service = WgVpnService.instance ?: throw IllegalStateException("VPN service not running")
        if (service.listTunnels().none { it.id == id }) {
            service.importTunnel(id, "User Config", config)
        }
        service.connectTunnel(id)
    }

    fun disconnectTunnel(id: String) {
        val serverID = tunnelToServerMap[id]
        if (serverID != null) {
            onManagedDisconnectRequest?.invoke(serverID)
        } else {
            viewModelScope.launch(Dispatchers.IO) {
                WgVpnService.instance?.disconnectTunnel(id)
            }
        }
    }

    fun importTunnel(name: String, config: String, onDone: (Boolean, String?) -> Unit) {
        viewModelScope.launch(Dispatchers.IO) {
            try {
                val service = WgVpnService.instance
                    ?: throw IllegalStateException("VPN service not running")

                if (settings.mode == "managed" && settings.token.isNotEmpty()) {
                    try {
                        val client = buildAuthClient()
                        val rawKey = client.getUserConfigKey()
                        val aad = settings.deviceID.toByteArray(Charsets.UTF_8)
                        val encrypted = DeviceCrypto.encryptBody(
                            rawKey, config.toByteArray(Charsets.UTF_8), aad
                        )
                        val encB64 = Base64.encodeToString(
                            encrypted.toByteArray(Charsets.UTF_8), Base64.NO_WRAP
                        )
                        val cfgId = client.uploadUserConfig(name, encB64)
                        val newTunnel = TunnelInfo(
                            id = "uconf-$cfgId", name = name,
                            status = TunnelStatus.DISCONNECTED,
                            addresses = emptyList(), dns = emptyList(),
                            mtu = null, listenPort = null, privateKey = "",
                            peers = emptyList(), isManaged = false, error = null
                        )
                        _uiState.update { s ->
                            s.copy(tunnels = s.tunnels + newTunnel, selectedTunnelId = newTunnel.id)
                        }
                        withContext(Dispatchers.Main) { onDone(true, null) }
                        return@launch
                    } catch (e: DeviceRevokedException) {
                        triggerAuthReset()
                        withContext(Dispatchers.Main) { onDone(false, "Login expired or revoked") }
                        return@launch
                    } catch (e: AuthInvalidException) {
                        triggerAuthReset()
                        withContext(Dispatchers.Main) { onDone(false, "Login expired or revoked") }
                        return@launch
                    } catch (_: Exception) { /* fall through to local */ }
                }

                val id = java.util.UUID.randomUUID().toString()
                val entry = service.importTunnel(id, name.ifEmpty { "Tunnel" }, config)
                val tunnel = buildTunnelInfo(entry)
                _uiState.update { s ->
                    s.copy(tunnels = s.tunnels + tunnel, selectedTunnelId = tunnel.id)
                }
                withContext(Dispatchers.Main) { onDone(true, null) }
            } catch (e: Exception) {
                withContext(Dispatchers.Main) { onDone(false, e.message) }
            }
        }
    }

    fun startDeleteCountdown(id: String) {
        deletePendingId = id
        deleteCountdownJob?.cancel()
        _uiState.update { it.copy(deleteCountdown = 3) }
        deleteCountdownJob = viewModelScope.launch {
            for (i in 2 downTo 0) {
                delay(1000)
                _uiState.update { it.copy(deleteCountdown = if (i > 0) i else null) }
            }
            confirmDelete()
        }
    }

    fun cancelDelete() {
        deleteCountdownJob?.cancel()
        deleteCountdownJob = null
        deletePendingId = null
        _uiState.update { it.copy(deleteCountdown = null) }
    }

    private fun confirmDelete() {
        val id = deletePendingId ?: return
        deletePendingId = null
        viewModelScope.launch(Dispatchers.IO) {
            try {
                if (id.startsWith("uconf-")) {
                    try { buildAuthClient().deleteUserConfig(id.removePrefix("uconf-")) }
                    catch (e: DeviceRevokedException) { triggerAuthReset(); return@launch }
                    catch (e: AuthInvalidException) { triggerAuthReset(); return@launch }
                    catch (_: Exception) {}
                }
                WgVpnService.instance?.deleteTunnel(id)
                _uiState.update { s ->
                    s.copy(
                        tunnels = s.tunnels.filter { it.id != id },
                        selectedTunnelId = if (s.selectedTunnelId == id) null else s.selectedTunnelId
                    )
                }
            } catch (e: Exception) { setError(e.message) }
        }
    }

    fun clearError() { _uiState.update { it.copy(error = null) } }

    fun clearAllLocalState() {
        statsJobs.values.forEach { it.cancel() }
        statsJobs.clear()
        deleteCountdownJob?.cancel()
        deleteCountdownJob = null
        deletePendingId = null
        WgVpnService.instance?.clearAllTunnels()
        _uiState.update { TunnelUiState() }
    }

    private fun triggerAuthReset() {
        viewModelScope.launch { onAuthInvalid?.invoke() }
    }

    // Called by ManagedViewModel after it removes its tunnel
    fun removeTunnelFromList(tunnelId: String) {
        stopStatsPolling(tunnelId)
        _uiState.update { s ->
            s.copy(
                tunnels = s.tunnels.filter { it.id != tunnelId },
                selectedTunnelId = if (s.selectedTunnelId == tunnelId) null else s.selectedTunnelId,
                stats = s.stats - tunnelId
            )
        }
    }

    // Called by ManagedViewModel when it adds a new managed tunnel
    fun addOrUpdateTunnel(tunnel: TunnelInfo) {
        _uiState.update { s ->
            val existing = s.tunnels.indexOfFirst { it.id == tunnel.id }
            val newList = if (existing >= 0) {
                s.tunnels.toMutableList().also { it[existing] = tunnel }
            } else {
                s.tunnels + tunnel
            }
            s.copy(tunnels = newList, selectedTunnelId = tunnel.id)
        }
    }

    private fun startStatsPolling(tunnelId: String) {
        stopStatsPolling(tunnelId)
        val job = viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                delay(2_000)
                val rawStats = WgVpnService.instance?.getStats(tunnelId) ?: continue
                var rx = 0L; var tx = 0L; var lastHandshake = 0L
                for (peer in rawStats.peers()) {
                    val ps = rawStats.peer(peer) ?: continue
                    rx += ps.rxBytes; tx += ps.txBytes
                    if (ps.latestHandshakeEpochMillis > lastHandshake)
                        lastHandshake = ps.latestHandshakeEpochMillis
                }
                val stats = StatsInfo(tunnelId, rx, tx, lastHandshake)
                _uiState.update { it.copy(stats = it.stats + (tunnelId to stats)) }
            }
        }
        statsJobs[tunnelId] = job
    }

    private fun stopStatsPolling(tunnelId: String) {
        statsJobs.remove(tunnelId)?.cancel()
        _uiState.update { it.copy(stats = it.stats - tunnelId) }
    }

    private fun setStatus(id: String, status: TunnelStatus) {
        _uiState.update { s ->
            s.copy(tunnels = s.tunnels.map { if (it.id == id) it.copy(status = status) else it })
        }
    }

    private fun setError(msg: String?) {
        _uiState.update { it.copy(error = msg) }
    }

    private fun buildAuthClient() = ManagedClient(
        baseURL = settings.serverURL,
        token = settings.token,
        deviceID = settings.deviceID,
        aesKey = if (settings.clientPrivateKey.isNotEmpty() && settings.serverPublicKey.isNotEmpty())
            DeviceCrypto.deriveAESKey(settings.clientPrivateKey, settings.serverPublicKey) else null
    )

    override fun onCleared() {
        super.onCleared()
        statsJobs.values.forEach { it.cancel() }
    }
}

// â”€â”€ Config parser â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

private data class ParsedConfig(
    val addresses: List<String>,
    val dns: List<String>,
    val mtu: Int?,
    val listenPort: Int?,
    val privateKey: String,
    val peers: List<PeerInfo>
)

private fun parseConfig(config: String): ParsedConfig {
    return try {
        val parsed = com.wireguard.config.Config.parse(BufferedReader(StringReader(config)))
        val iface = parsed.getInterface()
        val addresses = iface.addresses.map { it.toString() }
        val dns = iface.dnsServers.map { it.hostAddress ?: it.toString() }
        val mtu = iface.mtu.orElse(0).takeIf { it > 0 }
        val listenPort = iface.listenPort.orElse(0).takeIf { it > 0 }
        val peers = parsed.peers.map { peer ->
            PeerInfo(
                publicKey = "",
                endpoint = peer.endpoint.map { it.toString() }.orElse(null),
                allowedIps = peer.allowedIps.map { it.toString() },
                persistentKeepalive = peer.persistentKeepalive.orElse(0).takeIf { it > 0 }
            )
        }
        ParsedConfig(addresses, dns, mtu, listenPort, "", peers)
    } catch (_: Exception) {
        ParsedConfig(emptyList(), emptyList(), null, null, "", emptyList())
    }
}
