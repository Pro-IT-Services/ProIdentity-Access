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
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.R
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
                title = { Text(stringResource(R.string.common_settings)) },
                navigationIcon = {
                    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.common_back)) }
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
                SectionLabel(stringResource(R.string.settings_account))
                InfoCard {
                    if (settings.vpnName.isNotBlank()) InfoRow(stringResource(R.string.settings_organization), settings.vpnName)
                    InfoRow(stringResource(R.string.settings_server), hostOf(settings.serverUrl), mono = true)
                    InfoRow(stringResource(R.string.settings_signed_in_as), if (settings.loggedIn) settings.username else stringResource(R.string.settings_not_signed_in))
                }
                Spacer(Modifier.height(12.dp))
                if (settings.loggedIn) {
                    OutlinedButton(
                        onClick = { confirmSignOut = true },
                        shape = MaterialTheme.shapes.large,
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                    ) { Text(stringResource(R.string.common_sign_out)) }
                } else {
                    Button(
                        onClick = onSignIn,
                        shape = MaterialTheme.shapes.large,
                        modifier = Modifier.fillMaxWidth().height(52.dp),
                    ) { Text(stringResource(R.string.common_sign_in)) }
                }
                Spacer(Modifier.height(20.dp))
            }

            SectionLabel(stringResource(R.string.settings_this_device))
            InfoCard {
                InfoRow(stringResource(R.string.common_name), deviceName)
                InfoRow(stringResource(R.string.settings_mode), if (managed) stringResource(R.string.settings_mode_managed) else stringResource(R.string.settings_mode_standalone))
                if (managed) InfoRow(stringResource(R.string.settings_registration), if (settings.serverUrl.isNotBlank()) stringResource(R.string.settings_registered) else stringResource(R.string.settings_not_registered))
            }

            Spacer(Modifier.height(20.dp))
            SectionLabel(stringResource(R.string.settings_about))
            InfoCard {
                InfoRow(stringResource(R.string.settings_version), version, mono = true)
                ListItem(
                    headlineContent = { Text(stringResource(R.string.settings_website)) },
                    trailingContent = { Icon(Icons.AutoMirrored.Outlined.OpenInNew, contentDescription = null) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button) { uri.openUri(WEBSITE) },
                )
                ListItem(
                    headlineContent = { Text(stringResource(R.string.settings_privacy_policy)) },
                    trailingContent = { Icon(Icons.AutoMirrored.Outlined.OpenInNew, contentDescription = null) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button) { uri.openUri(PRIVACY_POLICY) },
                )
                ListItem(
                    headlineContent = { Text(stringResource(R.string.common_open_source_licenses)) },
                    colors = ListItemDefaults.colors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                    modifier = Modifier.clickable(role = Role.Button, onClick = onLicenses),
                )
            }

            Spacer(Modifier.height(28.dp))
            SectionLabel(stringResource(R.string.settings_reset_section))
            TextButton(
                onClick = { confirmReset = true },
                colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error),
                modifier = Modifier.fillMaxWidth().height(52.dp),
            ) { Text(stringResource(R.string.settings_reset_app)) }
            Text(
                stringResource(R.string.settings_reset_hint),
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
            title = { Text(stringResource(R.string.settings_sign_out_title)) },
            text = { Text(stringResource(R.string.settings_sign_out_body)) },
            confirmButton = { TextButton(onClick = { confirmSignOut = false; onSignOut() }) { Text(stringResource(R.string.common_sign_out)) } },
            dismissButton = { TextButton(onClick = { confirmSignOut = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }
    if (confirmReset) {
        AlertDialog(
            onDismissRequest = { confirmReset = false },
            title = { Text(stringResource(R.string.settings_reset_title)) },
            text = { Text(stringResource(R.string.settings_reset_body)) },
            confirmButton = {
                TextButton(
                    onClick = { confirmReset = false; onReset() },
                    colors = ButtonDefaults.textButtonColors(contentColor = MaterialTheme.colorScheme.error),
                ) { Text(stringResource(R.string.settings_reset_confirm)) }
            },
            dismissButton = { TextButton(onClick = { confirmReset = false }) { Text(stringResource(R.string.common_cancel)) } },
        )
    }
}
