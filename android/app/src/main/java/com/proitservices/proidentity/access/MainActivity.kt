package com.proitservices.proidentity.access

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.net.VpnService
import android.os.Build
import android.os.Bundle
import android.provider.OpenableColumns
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.animation.Crossfade
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.animation.core.tween
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.lifecycle.lifecycleScope
import com.proitservices.proidentity.access.ui.nav.AppNavHost
import com.proitservices.proidentity.access.ui.nav.ImportPrefill
import com.proitservices.proidentity.access.ui.screen.onboarding.OnboardingScreen
import com.proitservices.proidentity.access.ui.screen.onboarding.SignInScreen
import com.proitservices.proidentity.access.ui.theme.ProIdentityTheme
import com.proitservices.proidentity.access.ui.viewmodel.ManagedViewModel
import com.proitservices.proidentity.access.ui.viewmodel.SetupViewModel
import com.proitservices.proidentity.access.ui.viewmodel.StartRoute
import com.proitservices.proidentity.access.ui.viewmodel.TunnelViewModel
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    private val tunnelVm: TunnelViewModel by viewModels()
    private val setupVm: SetupViewModel by viewModels()
    private val managedVm: ManagedViewModel by viewModels()

    private val pendingImport = mutableStateOf<ImportPrefill?>(null)

    /** A managed connect waiting on the system VPN-permission dialog. */
    private var afterVpnPermission: (() -> Unit)? = null

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        val granted = result.resultCode == RESULT_OK
        tunnelVm.onVpnPermissionResult(granted)
        val pending = afterVpnPermission
        afterVpnPermission = null
        if (granted) pending?.invoke()
        askNotificationPermission()
    }

    /** Android 13+: needed to show the ongoing "Connected" notification. */
    private val notificationPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { }

    private fun askNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
    }

    private val filePicker = registerForActivityResult(ActivityResultContracts.GetContent()) { uri ->
        uri?.let { readConfig(it) }?.let { pendingImport.value = it }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)

        startService(Intent(this, WgVpnService::class.java))

        lifecycleScope.launch {
            tunnelVm.requestVpnPermission.collect { vpnPermissionLauncher.launch(it) }
        }

        // Cross-ViewModel wiring
        managedVm.onTunnelAdded = { tunnelVm.addOrUpdateTunnel(it) }
        managedVm.onTunnelRemoved = { tunnelVm.removeTunnelFromList(it) }
        tunnelVm.onManagedDisconnectRequest = { managedVm.disconnectServer(it) }
        // "Disconnect" in the ongoing notification takes the same path as the UI.
        WgVpnService.disconnectHandler = { tunnelVm.disconnectTunnel(it) }
        tunnelVm.onAuthFailure = { revoked -> managedVm.onAuthFailure(revoked) }
        tunnelVm.tunnelToServerMap = managedVm.tunnelServerIds

        setContent {
            ProIdentityTheme {
                var route by rememberSaveable { mutableStateOf(setupVm.startRoute()) }

                LaunchedEffect(Unit) {
                    when (route) {
                        StartRoute.ONBOARDING -> setupVm.resumeOnboarding()
                        StartRoute.SIGN_IN -> setupVm.beginReauth()
                        StartRoute.HOME -> {}
                    }
                }
                LaunchedEffect(Unit) {
                    setupVm.setupComplete.collect {
                        route = StartRoute.HOME
                        managedVm.reloadAfterSetup()
                        tunnelVm.refresh()
                    }
                }
                LaunchedEffect(Unit) {
                    // Device revoked by an admin, or the user reset the app.
                    managedVm.installationRevoked.collect {
                        tunnelVm.clearAllLocalState()
                        setupVm.resetWizard()
                        route = StartRoute.ONBOARDING
                    }
                }
                LaunchedEffect(Unit) {
                    // Session expired or signed out: keep device + server, ask for credentials.
                    managedVm.signInRequired.collect {
                        setupVm.beginReauth()
                        route = StartRoute.SIGN_IN
                    }
                }
                LaunchedEffect(route) {
                    // Ask for VPN permission once the user reaches their connections.
                    if (route == StartRoute.HOME) {
                        val consent = VpnService.prepare(this@MainActivity)
                        if (consent != null) vpnPermissionLauncher.launch(consent) else askNotificationPermission()
                    }
                }

                // Root surface: gives every screen the theme background and the
                // matching content color, so Text without an explicit color is
                // readable in both light and dark (the default is black).
                Surface(color = MaterialTheme.colorScheme.background, contentColor = MaterialTheme.colorScheme.onBackground) {
                    Crossfade(targetState = route, animationSpec = tween(280), label = "root") { current ->
                        when (current) {
                            StartRoute.ONBOARDING -> OnboardingScreen(setupVm)
                            StartRoute.SIGN_IN -> SignInScreen(setupVm, onContinueSignedOut = { route = StartRoute.HOME })
                            StartRoute.HOME -> AppNavHost(
                                tunnelVm = tunnelVm,
                                managedVm = managedVm,
                                pendingImport = pendingImport.value,
                                onImportConsumed = { pendingImport.value = null },
                                onPickFile = { filePicker.launch("*/*") },
                                withVpnPermission = ::withVpnPermission,
                                onSignIn = {
                                    setupVm.beginReauth()
                                    route = StartRoute.SIGN_IN
                                },
                            )
                        }
                    }
                }
            }
        }
    }

    override fun onDestroy() {
        // The view models go away with a finishing activity; the notification
        // then falls back to taking the tunnel down directly.
        if (isFinishing) WgVpnService.disconnectHandler = null
        super.onDestroy()
    }

    private fun withVpnPermission(action: () -> Unit) {
        val intent = VpnService.prepare(this)
        if (intent == null) action() else {
            afterVpnPermission = action
            vpnPermissionLauncher.launch(intent)
        }
    }

    /** Reads a picked config and its display name (extension stripped) for the import sheet. */
    private fun readConfig(uri: Uri): ImportPrefill? = runCatching {
        val content = contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() } ?: return null
        val name = contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            if (c.moveToFirst()) c.getString(0) else null
        }?.substringBeforeLast('.')
        ImportPrefill(name, content)
    }.getOrNull()
}
