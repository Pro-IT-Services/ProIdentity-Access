package com.proitservices.proidentity.access.ui.screen

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.automirrored.outlined.OpenInNew
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.model.ManagedSettings
import com.proitservices.proidentity.access.ui.design.InfoCard
import com.proitservices.proidentity.access.ui.design.InfoRow
import com.proitservices.proidentity.access.ui.design.SectionLabel
import com.proitservices.proidentity.access.ui.screen.onboarding.hostOf

private const val WEBSITE = "https://access.proidentity.cloud/"
private const val PRIVACY_POLICY = "https://access.proidentity.cloud/privacy"

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    settings: ManagedSettings,
    deviceName: String,
    version: String,
    onBack: () -> Unit,
    onSignIn: () -> Unit,
    onSignOut: () -> Unit,
    onReset: () -> Unit,
    onLicenses: () -> Unit,
) {
    val managed = settings.mode == "managed"
    var confirmSignOut by rememberSaveable { mutableStateOf(false) }
    var confirmReset by rememberSaveable { mutableStateOf(false) }
    val scroll = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val uri = LocalUriHandler.current

    Scaffold(
        modifier = Modifier.nestedScroll(scroll.nestedScrollConnection),
        topBar = {
            LargeTopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = "Back") }
                },
                scrollBehavior = scroll,
                colors = TopAppBarDefaults.largeTopAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        },
        containerColor = MaterialTheme.colorScheme.background,
    ) { padding ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .navigationBarsPadding()
                .padding(horizontal = 16.dp, vertical = 8.dp),
        ) {
            if (managed) {
                SectionLabel("Account")
                InfoCard {
                    if (settings.vpnName.isNotBlank()) InfoRow("Organization", settings.vpnName)
                    InfoRow("Server", hostOf(settings.serverUrl), mono = true)
                    InfoRow("Signed in as", if (settings.loggedIn) settings.username else "Not signed in")
                }
                Spacer(Modifier.height(12.dp))
                if (settings.loggedIn) {
                    OutlinedButton(
                        onClick = { confirmSignOut = true },
                        shape = MaterialTheme.shapes.large,
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                    ) { Text("Sign out") }
                } else {
                    Button(
                        onClick = onSignIn,
                        shape = MaterialTheme.shapes.large,
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                    ) { Text("Sign in") }
                }
                Spacer(Modifier.height(20.dp))
            }

            SectionLabel("This device")
            InfoCard {
                InfoRow("Name", deviceName)
                InfoRow("Mode", if (managed) "Managed by organization" else "Own configurations")
                if (managed) InfoRow("Registration", if (settings.serverUrl.isNotBlank()) "Registered" else "Not registered")
            }

            Spacer(Modifier.height(20.dp))
            SectionLabel("About")
            InfoCard {
                InfoRow("Version", version, mono = true)
                ListItem(
                    headlineContent = { Text("ProIdentity Access website") },
                    trailingContent = { Icon(Icons.AutoMirrored.Outlined.OpenInNew, contentDescription = null) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button) { uri.openUri(WEBSITE) },
                )
                ListItem(
                    headlineContent = { Text("Privacy policy") },
                    trailingContent = { Icon(Icons.AutoMirrored.Outlined.OpenInNew, contentDescription = null) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button) { uri.openUri(PRIVACY_POLICY) },
                )
                ListItem(
                    headlineContent = { Text("Open-source licenses") },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button, onClick = onLicenses),
                )
            }

            Spacer(Modifier.height(28.dp))
            SectionLabel("Reset")
            TextButton(
                onClick = { confirmReset = true },
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error),
                modifier = Modifier.fillMaxWidth().height(52.dp),
            ) { Text("Reset app") }
            Text(
                "Removes every connection and this device's registration from this phone.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
            Spacer(Modifier.height(24.dp))
        }
    }

    if (confirmSignOut) {
        AlertDialog(
            onDismissRequest = { confirmSignOut = false },
            title = { Text("Sign out?") },
            text = { Text("You'll be disconnected from managed servers. This device stays registered, so signing back in only needs your password.") },
            confirmButton = { TextButton(onClick = { confirmSignOut = false; onSignOut() }) { Text("Sign out") } },
            dismissButton = { TextButton(onClick = { confirmSignOut = false }) { Text("Cancel") } },
        )
    }
    if (confirmReset) {
        AlertDialog(
            onDismissRequest = { confirmReset = false },
            title = { Text("Reset ProIdentity Access?") },
            text = { Text("This disconnects everything and removes all configurations and the device registration from this phone. You'll need to set it up again.") },
            confirmButton = {
                TextButton(
                    onClick = { confirmReset = false; onReset() },
                    colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error),
                ) { Text("Reset") }
            },
            dismissButton = { TextButton(onClick = { confirmReset = false }) { Text("Cancel") } },
        )
    }
}
