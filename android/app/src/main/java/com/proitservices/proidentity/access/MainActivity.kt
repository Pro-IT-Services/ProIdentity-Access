package com.proitservices.proidentity.access

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.lifecycle.lifecycleScope
import com.proitservices.proidentity.access.ui.screen.MainScreen
import com.proitservices.proidentity.access.ui.screen.SetupScreen
import com.proitservices.proidentity.access.ui.theme.WgClientTheme
import com.proitservices.proidentity.access.ui.viewmodel.ManagedViewModel
import com.proitservices.proidentity.access.ui.viewmodel.SetupViewModel
import com.proitservices.proidentity.access.ui.viewmodel.TunnelViewModel
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    private val tunnelVm: TunnelViewModel by viewModels()
    private val setupVm: SetupViewModel by viewModels()
    private val managedVm: ManagedViewModel by viewModels()

    private val pendingFileContent = mutableStateOf<String?>(null)

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        tunnelVm.onVpnPermissionResult(result.resultCode == RESULT_OK)
    }

    private val filePicker = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        uri?.let {
            val content = contentResolver.openInputStream(it)?.bufferedReader()?.readText() ?: ""
            pendingFileContent.value = content
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        startService(Intent(this, WgVpnService::class.java))

        // Collect VPN permission requests from TunnelViewModel
        lifecycleScope.launch {
            tunnelVm.requestVpnPermission.collect { intent ->
                vpnPermissionLauncher.launch(intent)
            }
        }

        // Wire cross-ViewModel callbacks
        managedVm.onTunnelAdded = { tunnelVm.addOrUpdateTunnel(it) }
        managedVm.onTunnelRemoved = { tunnelVm.removeTunnelFromList(it) }
        managedVm.onLoginComplete = { tunnelVm.refresh() }
        tunnelVm.onManagedDisconnectRequest = { managedVm.disconnectServer(it) }
        tunnelVm.onAuthInvalid = { managedVm.forceAuthReset() }
        tunnelVm.tunnelToServerMap = managedVm.tunnelServerIds

        // Request VPN permission upfront
        val prepareIntent = VpnService.prepare(this)
        if (prepareIntent != null) vpnPermissionLauncher.launch(prepareIntent)

        setContent {
            WgClientTheme {
                var setupDone by remember { mutableStateOf(setupVm.checkSetup()) }
                val prefill by pendingFileContent

                LaunchedEffect(Unit) {
                    setupVm.setupComplete.collect {
                        setupDone = true
                        managedVm.reloadAfterSetup()
                        tunnelVm.refresh()
                    }
                }
                LaunchedEffect(Unit) {
                    managedVm.installationRevoked.collect {
                        tunnelVm.clearAllLocalState()
                        setupVm.resetWizard()
                        setupDone = false
                    }
                }

                if (!setupDone) {
                    SetupScreen(vm = setupVm)
                } else {
                    MainScreen(
                        tunnelVm = tunnelVm,
                        managedVm = managedVm,
                        onPickFile = { filePicker.launch("*/*") },
                        prefillConfig = prefill,
                        onPrefillConsumed = { pendingFileContent.value = null }
                    )
                }
            }
        }
    }
}
