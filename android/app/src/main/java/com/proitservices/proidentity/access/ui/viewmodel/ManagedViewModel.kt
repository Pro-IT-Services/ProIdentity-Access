package com.proitservices.proidentity.access.ui.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.proitservices.proidentity.access.WgVpnService
import com.proitservices.proidentity.access.bridge.AppSettings
import com.proitservices.proidentity.access.bridge.AuthInvalidException
import com.proitservices.proidentity.access.bridge.DeviceCrypto
import com.proitservices.proidentity.access.bridge.DeviceRevokedException
import com.proitservices.proidentity.access.bridge.EndpointCandidate
import com.proitservices.proidentity.access.bridge.LoginResponse
import com.proitservices.proidentity.access.bridge.ManagedClient
import com.proitservices.proidentity.access.bridge.ServerInfo
import com.proitservices.proidentity.access.model.ManagedSettings
import com.proitservices.proidentity.access.model.PeerInfo
import com.proitservices.proidentity.access.model.ServerStatus
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
import java.util.UUID

data class ManagedUiState(
    val settings: ManagedSettings = ManagedSettings(),
    val serverStatuses: Map<String, ServerStatus> = emptyMap(),
    val isLoading: Boolean = false,
    val showLoginModal: Boolean = false,
    val showTotpModal: Boolean = false,
    val showPushAuth: Boolean = false,
    val pushStatus: String = "idle",
    val pushAuthEnabled: Boolean = false,
    val totpTargetServerId: String? = null,
    val loginError: String? = null,
    val error: String? = null
)

class ManagedViewModel(application: Application) : AndroidViewModel(application) {

    private val appSettings = AppSettings(application)

    private val _uiState = MutableStateFlow(ManagedUiState())
    val uiState: StateFlow<ManagedUiState> = _uiState.asStateFlow()

    private val _installationRevoked = MutableSharedFlow<Unit>()
    val installationRevoked = _installationRevoked.asSharedFlow()

    private val _authExpired = MutableSharedFlow<Unit>()
    val authExpired = _authExpired.asSharedFlow()

    // Session tracking (was in AndroidBridge)
    private val activeSessions = mutableMapOf<String, String>()   // serverID â†’ sessionID
    private val serverTunnelIds = mutableMapOf<String, String>()  // serverID â†’ tunnelID
    val tunnelServerIds = mutableMapOf<String, String>()           // tunnelID â†’ serverID (read by TunnelViewModel)

    private var keepaliveJob: Job? = null
    private var serverPollJob: Job? = null

    // Callbacks wired by MainActivity after both VMs are created
    var onTunnelAdded: ((TunnelInfo) -> Unit)? = null
    var onTunnelRemoved: ((String) -> Unit)? = null
    var onLoginComplete: (() -> Unit)? = null  // triggers TunnelViewModel.refresh()

    init { loadSettings() }

    private fun loadSettings() {
        val s = appSettings
        val settings = ManagedSettings(
            serverUrl = s.serverURL,
            username = s.username,
            isAdmin = s.isAdmin,
            loggedIn = s.token.isNotEmpty(),
            vpnName = s.vpnName,
            totpEnabled = s.totpEnabled
        )
        _uiState.update { it.copy(settings = settings) }
        if (settings.loggedIn) {
            startServerPolling()
            startKeepaliveMonitor()
        }
    }

    fun reloadAfterSetup() {
        loadSettings()
        viewModelScope.launch(Dispatchers.IO) { fetchServers() }
    }

    fun openLoginModal() { _uiState.update { it.copy(showLoginModal = true, loginError = null) } }
    fun dismissLoginModal() { _uiState.update { it.copy(showLoginModal = false, loginError = null) } }
    fun dismissTotpModal() { _uiState.update { it.copy(showTotpModal = false, totpTargetServerId = null) } }
    fun clearError() { _uiState.update { it.copy(error = null) } }

    private var loginUsername = ""
    private var loginPassword = ""

    fun login(username: String, password: String, totpCode: String = "") {
        loginUsername = username
        loginPassword = password
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true, loginError = null) }
            try {
                val client = buildAuthClient()
                val resp = client.login(username, password, totpCode)
                if (resp.requireTotp) {
                    if (resp.pushAuthEnabled && resp.pushRequestId.isNotEmpty()) {
                        _uiState.update { it.copy(isLoading = false, showLoginModal = false, showPushAuth = true, pushStatus = "pending", pushAuthEnabled = true) }
                        pollPushLogin(resp.pushRequestId)
                    } else {
                        _uiState.update { it.copy(isLoading = false, loginError = "Enter your TOTP code", pushAuthEnabled = resp.pushAuthEnabled) }
                    }
                } else {
                    completeLogin(resp)
                }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, loginError = "Login failed: ${e.message}") }
            }
        }
    }

    fun loginWithPush(pushAuthID: String) {
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true) }
            try {
                val client = buildAuthClient()
                val resp = client.login(loginUsername, loginPassword, pushAuthID = pushAuthID)
                completeLogin(resp)
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, loginError = "Login failed: ${e.message}", showPushAuth = false) }
            }
        }
    }

    private fun pollPushLogin(requestID: String) {
        viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                delay(2000)
                try {
                    val client = buildAuthClient()
                    val status = client.pollPushStatus(requestID)
                    _uiState.update { it.copy(pushStatus = status) }
                    when (status) {
                        "approved" -> { loginWithPush(requestID); return@launch }
                        "denied", "expired" -> return@launch
                    }
                } catch (_: Exception) {}
            }
        }
    }

    fun dismissPushAuth() { _uiState.update { it.copy(showPushAuth = false, pushStatus = "idle") } }

    fun forceAuthReset() {
        handleAuthReset()
    }

    private fun completeLogin(resp: LoginResponse) {
        appSettings.token = resp.token
        appSettings.username = resp.username
        appSettings.isAdmin = resp.isAdmin
        appSettings.totpEnabled = resp.totpEnabled
        loadSettings()
        _uiState.update { it.copy(isLoading = false, showLoginModal = false, showPushAuth = false, pushStatus = "idle") }
        fetchServers()
        onLoginComplete?.invoke()
    }

    fun logout() {
        viewModelScope.launch(Dispatchers.IO) {
            stopServerPolling()
            stopKeepaliveMonitor()
            val serverIds = activeSessions.keys.toList()
            for (sid in serverIds) {
                try { disconnectServerInternal(sid) } catch (_: Exception) {}
            }
            appSettings.token = ""
            appSettings.username = ""
            appSettings.isAdmin = false
            loadSettings()
            _uiState.update { it.copy(serverStatuses = emptyMap()) }
        }
    }

    fun refreshServers() {
        viewModelScope.launch(Dispatchers.IO) { fetchServers() }
    }

    private fun fetchServers() {
        try {
            val servers = buildAuthClient().listServers()
            val statuses = servers.associate { server ->
                val tunnelId = serverTunnelIds[server.id]
                server.id to ServerStatus(
                    server = server,
                    connected = tunnelId != null,
                    tunnelId = tunnelId,
                    connecting = false,
                    error = null
                )
            }
            _uiState.update { it.copy(serverStatuses = statuses) }
        } catch (e: DeviceRevokedException) {
            handleAuthReset()
        } catch (e: AuthInvalidException) {
            handleAuthReset()
        } catch (_: Exception) {}
    }

    fun connectServer(serverId: String, totpCode: String = "", pushAuthID: String = "") {
        val status = _uiState.value.serverStatuses[serverId]
        val serverName = status?.server?.name ?: serverId

        viewModelScope.launch(Dispatchers.IO) {
            setServerStatus(serverId) { it.copy(connecting = true, error = null) }
            try {
                val client = buildAuthClient()
                val (wgPriv, wgPub) = DeviceCrypto.generateWireGuardKeypair()
                val sessionResp = client.createSession(serverId, wgPub, totpCode, pushAuthID)

                if (sessionResp.requireTotp) {
                    setServerStatus(serverId) { it.copy(connecting = false) }
                    if (sessionResp.pushAuthEnabled) {
                        _uiState.update { it.copy(showPushAuth = true, pushStatus = "pending", pushAuthEnabled = true, totpTargetServerId = serverId) }
                        val (reqId, _) = client.createPushAuth("Connect to $serverName")
                        pollPushConnect(serverId, reqId)
                    } else {
                        _uiState.update { it.copy(showTotpModal = true, totpTargetServerId = serverId, pushAuthEnabled = false) }
                    }
                    return@launch
                }

                var config = sessionResp.wgConfig
                config = injectPrivateKey(config, wgPriv)

                val service = WgVpnService.instance ?: throw IllegalStateException("VPN service not running")
                val tunnelId = connectEndpointCandidates(service, serverName, config, sessionResp.endpoints, serverId)

                activeSessions[serverId] = sessionResp.sessionId
                serverTunnelIds[serverId] = tunnelId
                tunnelServerIds[tunnelId] = serverId

                val tunnel = TunnelInfo(
                    id = tunnelId, name = serverName,
                    status = TunnelStatus.CONNECTED,
                    addresses = emptyList(), dns = emptyList(),
                    mtu = null, listenPort = null, privateKey = "",
                    peers = emptyList(), isManaged = true, error = null
                )
                onTunnelAdded?.invoke(tunnel)
                setServerStatus(serverId) { it.copy(connected = true, tunnelId = tunnelId, connecting = false) }
            } catch (e: DeviceRevokedException) {
                handleAuthReset()
            } catch (e: AuthInvalidException) {
                handleAuthReset()
            } catch (e: Exception) {
                setServerStatus(serverId) { it.copy(connecting = false, error = e.message) }
            }
        }
    }

    fun connectServerWithTotp(totpCode: String) {
        val serverId = _uiState.value.totpTargetServerId ?: return
        _uiState.update { it.copy(showTotpModal = false, totpTargetServerId = null) }
        connectServer(serverId, totpCode)
    }

    private fun pollPushConnect(serverId: String, requestID: String) {
        viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                delay(2000)
                try {
                    val client = buildAuthClient()
                    val status = client.pollPushStatus(requestID)
                    _uiState.update { it.copy(pushStatus = status) }
                    when (status) {
                        "approved" -> {
                            _uiState.update { it.copy(showPushAuth = false, pushStatus = "idle") }
                            connectServer(serverId, pushAuthID = requestID)
                            return@launch
                        }
                        "denied", "expired" -> return@launch
                    }
                } catch (_: Exception) {}
            }
        }
    }

    fun disconnectServer(serverId: String) {
        viewModelScope.launch(Dispatchers.IO) {
            try { disconnectServerInternal(serverId) }
            catch (e: Exception) { _uiState.update { it.copy(error = e.message) } }
        }
    }

    fun disconnectByTunnelId(tunnelId: String) {
        val serverId = tunnelServerIds[tunnelId]
        if (serverId != null) disconnectServer(serverId)
        else viewModelScope.launch(Dispatchers.IO) {
            WgVpnService.instance?.deleteTunnel(tunnelId)
        }
    }

    private fun disconnectServerInternal(serverId: String) {
        val sessionId = activeSessions.remove(serverId)
        val tunnelId = serverTunnelIds.remove(serverId)
        if (tunnelId != null) tunnelServerIds.remove(tunnelId)

        if (sessionId != null) {
            try { buildAuthClient().deleteSession(sessionId) } catch (_: Exception) {}
        }
        if (tunnelId != null) {
            WgVpnService.instance?.deleteTunnel(tunnelId)
            onTunnelRemoved?.invoke(tunnelId)
        }
        setServerStatus(serverId) { it.copy(connected = false, tunnelId = null, connecting = false, error = null) }
    }

    private fun startServerPolling() {
        stopServerPolling()
        serverPollJob = viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                fetchServers()
                delay(10_000)
            }
        }
    }

    private fun stopServerPolling() { serverPollJob?.cancel(); serverPollJob = null }

    private fun startKeepaliveMonitor() {
        stopKeepaliveMonitor()
        keepaliveJob = viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                delay(10_000)
                // Auth check â€” detect password change, user disabled, etc.
                try {
                    buildAuthClient().checkAuth()
                } catch (e: DeviceRevokedException) {
                    handleAuthReset(); return@launch
                } catch (e: AuthInvalidException) {
                    handleAuthReset(); return@launch
                } catch (_: Exception) {
                    return@launch
                }
                // Keepalives
                val sessions = activeSessions.entries.toList()
                for ((_, sessionId) in sessions) {
                    try {
                    buildAuthClient().keepalive(sessionId)
                } catch (e: DeviceRevokedException) {
                    handleAuthReset(); return@launch
                } catch (e: AuthInvalidException) {
                    handleAuthReset(); return@launch
                } catch (_: Exception) {}
            }
        }
        }
    }

    private fun stopKeepaliveMonitor() { keepaliveJob?.cancel(); keepaliveJob = null }

    private fun handleAuthReset() {
        stopServerPolling()
        stopKeepaliveMonitor()
        WgVpnService.instance?.clearAllTunnels()
        appSettings.wipeAll()
        activeSessions.clear()
        serverTunnelIds.clear()
        tunnelServerIds.clear()
        _uiState.update { ManagedUiState(error = "Login expired or revoked. Please set up again.") }
        viewModelScope.launch {
            _authExpired.emit(Unit)
            _installationRevoked.emit(Unit)
        }
    }

    private fun setServerStatus(serverId: String, update: (ServerStatus) -> ServerStatus) {
        _uiState.update { s ->
            val current = s.serverStatuses[serverId] ?: return@update s
            s.copy(serverStatuses = s.serverStatuses + (serverId to update(current)))
        }
    }

    private fun buildAuthClient() = ManagedClient(
        baseURL = appSettings.serverURL,
        token = appSettings.token,
        deviceID = appSettings.deviceID,
        aesKey = if (appSettings.clientPrivateKey.isNotEmpty() && appSettings.serverPublicKey.isNotEmpty())
            DeviceCrypto.deriveAESKey(appSettings.clientPrivateKey, appSettings.serverPublicKey) else null
    )

    override fun onCleared() {
        super.onCleared()
        keepaliveJob?.cancel()
        serverPollJob?.cancel()
    }
}

private fun injectPrivateKey(config: String, privKeyB64: String): String {
    val lines = config.lines().toMutableList()
    var inInterface = false
    var privKeyIndex = -1
    var interfaceIndex = -1
    for (i in lines.indices) {
        val trimmed = lines[i].trim()
        if (trimmed.equals("[Interface]", ignoreCase = true)) {
            inInterface = true; interfaceIndex = i
        } else if (trimmed.startsWith("[") && i != interfaceIndex) {
            inInterface = false
        }
        if (inInterface && trimmed.startsWith("PrivateKey", ignoreCase = true)) {
            privKeyIndex = i; break
        }
    }
    return if (privKeyIndex >= 0) {
        lines[privKeyIndex] = "PrivateKey = $privKeyB64"
        lines.joinToString("\n")
    } else if (interfaceIndex >= 0) {
        lines.add(interfaceIndex + 1, "PrivateKey = $privKeyB64")
        lines.joinToString("\n")
    } else config
}

private fun connectEndpointCandidates(
    service: WgVpnService,
    serverName: String,
    config: String,
    endpoints: List<EndpointCandidate>,
    serverId: String
): String {
    val configs = endpointConfigs(config, endpoints)
    var lastError: Exception? = null
    configs.forEach { candidateConfig ->
        val tunnelId = "managed-$serverId-${UUID.randomUUID()}"
        try {
            service.importTunnel(tunnelId, serverName, candidateConfig)
            service.connectTunnel(tunnelId)
            return tunnelId
        } catch (e: Exception) {
            lastError = e
            try { service.deleteTunnel(tunnelId) } catch (_: Exception) {}
        }
    }
    throw IllegalStateException(lastError?.message ?: "No endpoint candidates could connect")
}

private fun endpointConfigs(config: String, endpoints: List<EndpointCandidate>): List<String> {
    val seen = mutableSetOf<String>()
    val configs = endpoints.mapNotNull { ep ->
        val endpoint = ep.endpoint.ifBlank {
            if (ep.ip.isNotBlank() && ep.port > 0) "${ep.ip}:${ep.port}" else ""
        }
        if (endpoint.isBlank() || !seen.add(endpoint)) null else replaceEndpoint(config, endpoint)
    }
    return configs.ifEmpty { listOf(config) }
}

private fun replaceEndpoint(config: String, endpoint: String): String {
    val lines = config.lines().toMutableList()
    for (i in lines.indices) {
        if (lines[i].trim().startsWith("Endpoint", ignoreCase = true)) {
            lines[i] = "Endpoint = $endpoint"
            return lines.joinToString("\n")
        }
    }
    return config
}
