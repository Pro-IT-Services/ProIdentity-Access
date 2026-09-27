package com.proitservices.proidentity.access.ui.sheet

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.FileOpen
import androidx.compose.material.icons.outlined.PhonelinkRing
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.ui.screen.onboarding.OtpField
import com.proitservices.proidentity.access.ui.theme.Brand
import com.proitservices.proidentity.access.ui.theme.MonoStyle

/**
 * 2FA prompt shown when connecting to a managed server that requires it.
 * Push mode: waits for approval on the phone. Code mode: 6-digit entry.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ConnectAuthSheet(
    serverName: String,
    pushMode: Boolean,
    pushStatus: String,
    pushAvailable: Boolean,
    onSubmitCode: (String) -> Unit,
    onRetryPush: () -> Unit,
    onUseCode: () -> Unit,
    onDismiss: () -> Unit,
) {
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .imePadding()
                .padding(horizontal = 24.dp)
                .padding(bottom = 24.dp),
        ) {
            if (pushMode) PushContent(serverName, pushStatus, onRetryPush, onUseCode)
            else CodeContent(serverName, pushAvailable, onSubmitCode, onRetryPush)
        }
    }
}

@Composable
private fun PushContent(serverName: String, status: String, onRetry: () -> Unit, onUseCode: () -> Unit) {
    val failed = status == "denied" || status == "expired"
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier.size(52.dp).clip(CircleShape)
                .background(if (failed) Brand.status.warningContainer else MaterialTheme.colorScheme.primaryContainer),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                Icons.Outlined.PhonelinkRing, contentDescription = null,
                tint = if (failed) Brand.status.warning else MaterialTheme.colorScheme.primary,
            )
        }
        Spacer(Modifier.width(16.dp))
        Column {
            Text(
                when (status) {
                    "denied" -> "Request denied"
                    "expired" -> "Request expired"
                    else -> "Approve on your phone"
                },
                style = MaterialTheme.typography.titleLarge,
                modifier = Modifier.semantics { heading() },
            )
            Text("Connecting to $serverName", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
    Spacer(Modifier.height(20.dp))
    if (failed) {
        Button(onClick = onRetry, shape = MaterialTheme.shapes.large, modifier = Modifier.fillMaxWidth().height(52.dp)) {
            Text("Send again")
        }
    } else {
        Row(verticalAlignment = Alignment.CenterVertically) {
            CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
            Spacer(Modifier.width(12.dp))
            Text(
                "Check the ProIdentity app for a notification.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
    Spacer(Modifier.height(8.dp))
    TextButton(onClick = onUseCode, modifier = Modifier.heightIn(min = 48.dp)) { Text("Enter a code instead") }
}

@Composable
private fun CodeContent(serverName: String, pushAvailable: Boolean, onSubmit: (String) -> Unit, onUsePush: () -> Unit) {
    var code by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(code) { if (code.length == 6) onSubmit(code) }
    Text("Verification required", style = MaterialTheme.typography.titleLarge, modifier = Modifier.semantics { heading() })
    Spacer(Modifier.height(4.dp))
    Text(
        "Enter the 6-digit code from your authenticator app to connect to $serverName.",
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
    Spacer(Modifier.height(20.dp))
    OtpField(value = code, onValueChange = { code = it })
    Spacer(Modifier.height(16.dp))
    Button(
        onClick = { onSubmit(code) },
        enabled = code.length == 6,
        shape = MaterialTheme.shapes.large,
        modifier = Modifier.fillMaxWidth().height(52.dp),
    ) { Text("Verify and connect") }
    if (pushAvailable) {
        TextButton(onClick = onUsePush, modifier = Modifier.heightIn(min = 48.dp)) { Text("Use a push notification instead") }
    }
}

/** Import a WireGuard configuration from a file or by pasting it. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ImportSheet(
    prefillName: String?,
    prefillConfig: String?,
    loading: Boolean,
    error: String?,
    onPickFile: () -> Unit,
    onImport: (name: String, config: String) -> Unit,
    onDismiss: () -> Unit,
) {
    var name by rememberSaveable { mutableStateOf("") }
    var config by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(prefillConfig, prefillName) {
        if (!prefillConfig.isNullOrEmpty()) config = prefillConfig
        if (!prefillName.isNullOrEmpty() && name.isBlank()) name = prefillName
    }
    val sheet = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    ModalBottomSheet(onDismissRequest = onDismiss, sheetState = sheet) {
        Column(
            Modifier
                .fillMaxWidth()
                .navigationBarsPadding()
                .imePadding()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 24.dp)
                .padding(bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            Column {
                Text("Import configuration", style = MaterialTheme.typography.titleLarge, modifier = Modifier.semantics { heading() })
                Text(
                    "Choose a WireGuard .conf file, or paste its contents.",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            OutlinedButton(onClick = onPickFile, shape = MaterialTheme.shapes.large, modifier = Modifier.fillMaxWidth().height(52.dp)) {
                Icon(Icons.Outlined.FileOpen, contentDescription = null)
                Spacer(Modifier.width(10.dp))
                Text("Choose file")
            }
            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text("Name") },
                placeholder = { Text("Home office") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = config,
                onValueChange = { config = it },
                label = { Text("Configuration") },
                placeholder = { Text("[Interface]\n…\n[Peer]\n…", style = MonoStyle) },
                textStyle = MonoStyle,
                minLines = 6,
                maxLines = 12,
                isError = error != null,
                supportingText = error?.let { { Text(it) } },
                modifier = Modifier.fillMaxWidth(),
            )
            Button(
                onClick = { onImport(name.trim().ifEmpty { "Tunnel" }, config) },
                enabled = config.isNotBlank() && !loading,
                shape = MaterialTheme.shapes.large,
                modifier = Modifier.fillMaxWidth().height(52.dp),
            ) {
                if (loading) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
                else Text("Import")
            }
        }
    }
}
