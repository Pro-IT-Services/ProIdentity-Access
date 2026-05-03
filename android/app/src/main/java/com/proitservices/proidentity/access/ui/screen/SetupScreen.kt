package com.proitservices.proidentity.access.ui.screen

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.ui.viewmodel.SetupStep
import com.proitservices.proidentity.access.ui.viewmodel.SetupViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SetupScreen(vm: SetupViewModel, fromSettings: Boolean = false, onBack: () -> Unit = {}) {
    val state by vm.uiState.collectAsState()

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Setup", fontWeight = FontWeight.SemiBold) },
                navigationIcon = {
                    if (fromSettings || state.step != SetupStep.MODE) {
                        IconButton(onClick = {
                            if (state.step == SetupStep.MODE) onBack()
                            else vm.goBack()
                        }) {
                            Icon(Icons.Default.ArrowBack, contentDescription = "Back")
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.surface
                )
            )
        }
    ) { padding ->
        AnimatedContent(
            targetState = state.step,
            transitionSpec = {
                slideInHorizontally { it } togetherWith slideOutHorizontally { -it }
            },
            modifier = Modifier.padding(padding)
        ) { step ->
            when (step) {
                SetupStep.MODE     -> ModeStep(vm)
                SetupStep.SERVER   -> ServerStep(vm)
                SetupStep.REGISTER -> RegisterStep(vm)
                SetupStep.LOGIN    -> LoginStep(vm)
                SetupStep.DONE     -> {}
            }
        }
    }
}

@Composable
private fun ModeStep(vm: SetupViewModel) {
    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Icon(Icons.Default.Shield, contentDescription = null,
            modifier = Modifier.size(64.dp),
            tint = MaterialTheme.colorScheme.primary)
        Spacer(Modifier.height(16.dp))
        Text("ProIdentity Access", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text("Choose how you want to use this app",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(48.dp))

        ModeCard(
            title = "Standalone",
            description = "Import WireGuard .conf files manually",
            onClick = { vm.chooseStandalone() }
        )
        Spacer(Modifier.height(16.dp))
        ModeCard(
            title = "Managed",
            description = "Connect to a ProIdentity Access server",
            onClick = { vm.chooseManaged() }
        )
    }
}

@Composable
private fun ModeCard(title: String, description: String, onClick: () -> Unit) {
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Column(Modifier.padding(20.dp)) {
            Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(4.dp))
            Text(description, style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun ServerStep(vm: SetupViewModel) {
    val state by vm.uiState.collectAsState()
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Management Server", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text("Enter the URL of your ProIdentity Access server",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(32.dp))

        OutlinedTextField(
            value = state.serverUrl,
            onValueChange = { vm.setServerUrl(it) },
            label = { Text("Server URL") },
            placeholder = { Text("https://vpn.example.com") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
            keyboardActions = KeyboardActions(onGo = { vm.submitServerUrl() })
        )

        state.error?.let {
            Spacer(Modifier.height(8.dp))
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        Spacer(Modifier.height(24.dp))
        Button(onClick = { vm.submitServerUrl() }, modifier = Modifier.fillMaxWidth(), enabled = !state.isLoading) {
            if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
            else Text("Continue")
        }
    }
}

@Composable
private fun RegisterStep(vm: SetupViewModel) {
    val state by vm.uiState.collectAsState()
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text("Register Device", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        Text("Give this device a name to identify it on the server",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(32.dp))

        OutlinedTextField(
            value = state.deviceName,
            onValueChange = { vm.setDeviceName(it) },
            label = { Text("Device Name") },
            placeholder = { Text("My Phone") },
            modifier = Modifier.fillMaxWidth(),
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Go),
            keyboardActions = KeyboardActions(onGo = { vm.registerDevice() })
        )

        state.error?.let {
            Spacer(Modifier.height(8.dp))
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        Spacer(Modifier.height(24.dp))
        Button(onClick = { vm.registerDevice() }, modifier = Modifier.fillMaxWidth(), enabled = !state.isLoading) {
            if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
            else Text("Register")
        }
    }
}

@Composable
private fun LoginStep(vm: SetupViewModel) {
    val state by vm.uiState.collectAsState()
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Default.Lock, contentDescription = null,
                tint = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.size(8.dp))
            Text(
                when (state.loginMode) {
                    "push" -> "Push Verification"
                    "totp" -> "Two-Factor Auth"
                    else -> "Sign In"
                },
                style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold
            )
        }
        Spacer(Modifier.height(32.dp))

        if (state.loginMode == "push") {
            Column(
                modifier = Modifier.fillMaxWidth(),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                if (state.pushStatus == "pending") {
                    CircularProgressIndicator(Modifier.size(48.dp))
                    Text("Waiting for approvalâ€¦", style = MaterialTheme.typography.titleMedium)
                    Text("Check your phone for a push notification.",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        textAlign = TextAlign.Center)
                } else if (state.pushStatus == "approved") {
                    CircularProgressIndicator(Modifier.size(48.dp))
                    Text("Approved â€” signing inâ€¦", style = MaterialTheme.typography.titleMedium)
                } else {
                    Text(if (state.pushStatus == "denied") "Denied" else "Expired",
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.error)
                    Button(onClick = { vm.switchToPush() }) { Text("Try again") }
                }
                if (state.pushStatus != "approved") {
                    TextButton(onClick = { vm.switchToTotp() }) { Text("Enter code manually") }
                }
            }
        } else if (state.loginMode == "totp") {
            OutlinedTextField(
                value = state.totpCode, onValueChange = { vm.setTotpCode(it) },
                label = { Text("6-digit code") }, modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number, imeAction = ImeAction.Go),
                keyboardActions = KeyboardActions(onGo = { vm.loginWithTotp() })
            )
            state.error?.let {
                Spacer(Modifier.height(8.dp))
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            Spacer(Modifier.height(24.dp))
            Button(onClick = { vm.loginWithTotp() }, modifier = Modifier.fillMaxWidth(), enabled = !state.isLoading) {
                if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Text("Verify")
            }
            if (state.pushAuthEnabled) {
                Spacer(Modifier.height(8.dp))
                TextButton(onClick = { vm.switchToPush() }, modifier = Modifier.fillMaxWidth()) {
                    Text("Use push notification instead")
                }
            }
        } else {
            OutlinedTextField(
                value = state.username, onValueChange = { vm.setUsername(it) },
                label = { Text("Username") }, modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = state.password, onValueChange = { vm.setPassword(it) },
                label = { Text("Password") }, modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Go),
                keyboardActions = KeyboardActions(onGo = { vm.login() })
            )
            state.error?.let {
                Spacer(Modifier.height(8.dp))
                Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            Spacer(Modifier.height(24.dp))
            Button(onClick = { vm.login() }, modifier = Modifier.fillMaxWidth(), enabled = !state.isLoading) {
                if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Text("Sign In")
            }
        }
    }
}
