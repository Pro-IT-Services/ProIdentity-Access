package com.proitservices.proidentity.access.ui.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.proitservices.proidentity.access.bridge.AppSettings
import com.proitservices.proidentity.access.bridge.DeviceCrypto
import com.proitservices.proidentity.access.bridge.ManagedClient
import com.proitservices.proidentity.access.bridge.defaultDeviceName
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

enum class SetupStep { MODE, SERVER, REGISTER, LOGIN, DONE }

data class SetupUiState(
    val step: SetupStep = SetupStep.MODE,
    val serverUrl: String = "",
    val deviceName: String = "",
    val username: String = "",
    val password: String = "",
    val totpCode: String = "",
    val requireTotp: Boolean = false,
    val pushAuthEnabled: Boolean = false,
    val pushStatus: String = "idle",
    val loginMode: String = "credentials", // "credentials", "totp", "push"
    val isLoading: Boolean = false,
    val error: String? = null
)

class SetupViewModel(application: Application) : AndroidViewModel(application) {

    private val settings = AppSettings(application)

    private val _uiState = MutableStateFlow(SetupUiState(deviceName = defaultDeviceName()))
    val uiState: StateFlow<SetupUiState> = _uiState.asStateFlow()

    private val _setupComplete = MutableSharedFlow<Unit>()
    val setupComplete = _setupComplete.asSharedFlow()

    fun checkSetup(): Boolean = settings.setupDone

    fun resetWizard() {
        _uiState.update { SetupUiState(step = SetupStep.MODE, deviceName = defaultDeviceName()) }
    }

    fun determineStartStep(): SetupStep {
        if (settings.setupDone) return SetupStep.DONE
        if (settings.mode.isEmpty()) return SetupStep.MODE
        if (settings.mode == "standalone") return SetupStep.DONE
        if (settings.deviceID.isEmpty()) return SetupStep.REGISTER
        if (settings.token.isEmpty()) return SetupStep.LOGIN
        return SetupStep.DONE
    }

    fun setServerUrl(url: String) { _uiState.update { it.copy(serverUrl = url) } }
    fun setDeviceName(name: String) { _uiState.update { it.copy(deviceName = name) } }
    fun setUsername(u: String) { _uiState.update { it.copy(username = u) } }
    fun setPassword(p: String) { _uiState.update { it.copy(password = p) } }
    fun setTotpCode(t: String) { _uiState.update { it.copy(totpCode = t) } }
    fun clearError() { _uiState.update { it.copy(error = null) } }

    fun chooseStandalone() {
        settings.mode = "standalone"
        settings.setupDone = true
        viewModelScope.launch { _setupComplete.emit(Unit) }
    }

    fun chooseManaged() {
        settings.mode = "managed"
        _uiState.update { it.copy(step = SetupStep.SERVER) }
    }

    fun submitServerUrl() {
        val url = _uiState.value.serverUrl.trim()
        if (url.isEmpty()) {
            _uiState.update { it.copy(error = "Server URL cannot be empty") }
            return
        }
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val client = ManagedClient(baseURL = url)
                val info = client.getInfo()
                settings.serverURL = url
                settings.vpnName = info.vpnName
                _uiState.update {
                    it.copy(
                        isLoading = false,
                        step = SetupStep.REGISTER,
                        deviceName = it.deviceName.ifBlank { defaultDeviceName() }
                    )
                }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, error = "Cannot reach server: ${e.message}") }
            }
        }
    }

    fun registerDevice() {
        val name = _uiState.value.deviceName.trim().ifEmpty { defaultDeviceName() }
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val (privB64, pubB64) = DeviceCrypto.generateX25519KeyPair()
                val client = ManagedClient(baseURL = settings.serverURL)
                val resp = client.register(name, pubB64)
                settings.clientPrivateKey = privB64
                settings.serverPublicKey = resp.serverPublicKey
                settings.deviceID = resp.deviceId
                _uiState.update { it.copy(isLoading = false, step = SetupStep.LOGIN) }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, error = "Registration failed: ${e.message}") }
            }
        }
    }

    fun login() {
        val s = _uiState.value
        if (s.username.isEmpty() || s.password.isEmpty()) {
            _uiState.update { it.copy(error = "Username and password required") }
            return
        }
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val client = buildClient()
                val resp = client.login(s.username, s.password, s.totpCode)
                if (resp.requireTotp) {
                    if (resp.pushAuthEnabled && resp.pushRequestId.isNotEmpty()) {
                        _uiState.update { it.copy(isLoading = false, requireTotp = true, pushAuthEnabled = true, loginMode = "push", pushStatus = "pending") }
                        pollPushLogin(client, resp.pushRequestId)
                    } else {
                        _uiState.update { it.copy(isLoading = false, requireTotp = true, pushAuthEnabled = resp.pushAuthEnabled, loginMode = "totp", totpCode = "", error = null) }
                    }
                } else {
                    finishLogin(resp)
                }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, error = "Login failed: ${e.message}") }
            }
        }
    }

    fun loginWithTotp() {
        val s = _uiState.value
        viewModelScope.launch(Dispatchers.IO) {
            _uiState.update { it.copy(isLoading = true, error = null) }
            try {
                val client = buildClient()
                val resp = client.login(s.username, s.password, s.totpCode)
                if (resp.requireTotp) {
                    _uiState.update { it.copy(isLoading = false, error = "Invalid code") }
                } else {
                    finishLogin(resp)
                }
            } catch (e: Exception) {
                _uiState.update { it.copy(isLoading = false, error = "Login failed: ${e.message}") }
            }
        }
    }

    fun switchToTotp() { _uiState.update { it.copy(loginMode = "totp", error = null) } }
    fun switchToPush() {
        _uiState.update { it.copy(loginMode = "push", pushStatus = "pending", error = null) }
        login() // re-trigger to get a new push request
    }

    private fun pollPushLogin(client: ManagedClient, requestID: String) {
        viewModelScope.launch(Dispatchers.IO) {
            while (isActive) {
                delay(2000)
                try {
                    val status = client.pollPushStatus(requestID)
                    _uiState.update { it.copy(pushStatus = status) }
                    when (status) {
                        "approved" -> {
                            _uiState.update { it.copy(isLoading = true) }
                            val s = _uiState.value
                            val resp = client.login(s.username, s.password, pushAuthID = requestID)
                            finishLogin(resp)
                            return@launch
                        }
                        "denied", "expired" -> return@launch
                    }
                } catch (_: Exception) {}
            }
        }
    }

    private fun finishLogin(resp: com.proitservices.proidentity.access.bridge.LoginResponse) {
        settings.token = resp.token
        settings.username = resp.username
        settings.isAdmin = resp.isAdmin
        settings.totpEnabled = resp.totpEnabled
        settings.setupDone = true
        _uiState.update { it.copy(isLoading = false, pushStatus = "idle") }
        viewModelScope.launch { _setupComplete.emit(Unit) }
    }

    private fun buildClient(): ManagedClient {
        val aesKey = if (settings.clientPrivateKey.isNotEmpty() && settings.serverPublicKey.isNotEmpty())
            DeviceCrypto.deriveAESKey(settings.clientPrivateKey, settings.serverPublicKey) else null
        return ManagedClient(
            baseURL = settings.serverURL,
            deviceID = settings.deviceID,
            aesKey = aesKey
        )
    }

    fun goBack() {
        _uiState.update { s ->
            s.copy(
                error = null,
                step = when (s.step) {
                    SetupStep.SERVER   -> SetupStep.MODE
                    SetupStep.REGISTER -> SetupStep.SERVER
                    SetupStep.LOGIN    -> SetupStep.REGISTER
                    else               -> s.step
                }
            )
        }
    }
}
