package com.proitservices.proidentity.access.ui.component

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.PhoneAndroid
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp

@Composable
fun TotpModal(
    onSubmit: (code: String) -> Unit,
    onDismiss: () -> Unit,
    pushAuthEnabled: Boolean = false,
    pushStatus: String = "idle",
    onSwitchToPush: (() -> Unit)? = null,
    onSwitchToTotp: (() -> Unit)? = null,
    onRetryPush: (() -> Unit)? = null
) {
    var mode by remember(pushAuthEnabled) { mutableStateOf(if (pushAuthEnabled && pushStatus != "idle") "push" else "totp") }
    var code by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (mode == "push") "Push Verification" else "Two-Factor Authentication") },
        text = {
            if (mode == "push") {
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    if (pushStatus == "approved") {
                        Icon(Icons.Default.CheckCircle, contentDescription = null,
                            modifier = Modifier.size(48.dp),
                            tint = MaterialTheme.colorScheme.primary)
                        Text("Approved â€” connectingâ€¦",
                            style = MaterialTheme.typography.titleMedium)
                    } else {
                        if (pushStatus == "pending") {
                            CircularProgressIndicator(modifier = Modifier.size(48.dp))
                        } else {
                            Icon(Icons.Default.PhoneAndroid, contentDescription = null,
                                modifier = Modifier.size(48.dp),
                                tint = MaterialTheme.colorScheme.error)
                        }
                        Text(
                            when (pushStatus) {
                                "pending" -> "Waiting for approvalâ€¦"
                                "denied" -> "Denied"
                                "expired" -> "Expired"
                                else -> "Waitingâ€¦"
                            },
                            style = MaterialTheme.typography.titleMedium
                        )
                        Text(
                            when (pushStatus) {
                                "pending" -> "Check your phone for a push notification."
                                "denied" -> "The request was denied."
                                "expired" -> "The request expired."
                                else -> ""
                            },
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            textAlign = TextAlign.Center
                        )
                    }
                    if (pushStatus == "denied" || pushStatus == "expired") {
                        Button(onClick = { onRetryPush?.invoke() }) { Text("Try again") }
                    }
                    if (pushStatus != "approved") {
                        TextButton(onClick = { mode = "totp"; onSwitchToTotp?.invoke() }) {
                            Text("Enter code manually")
                        }
                    }
                }
            } else {
                Column(modifier = Modifier.fillMaxWidth()) {
                    OutlinedTextField(
                        value = code,
                        onValueChange = { if (it.length <= 6 && it.all(Char::isDigit)) code = it },
                        label = { Text("6-digit code") },
                        modifier = Modifier.fillMaxWidth(),
                        singleLine = true,
                        keyboardOptions = KeyboardOptions(
                            keyboardType = KeyboardType.Number,
                            imeAction = ImeAction.Go
                        ),
                        keyboardActions = KeyboardActions(onGo = { if (code.length == 6) onSubmit(code) })
                    )
                    if (pushAuthEnabled) {
                        Spacer(Modifier.height(8.dp))
                        TextButton(
                            onClick = { mode = "push"; onSwitchToPush?.invoke() },
                            modifier = Modifier.align(Alignment.CenterHorizontally)
                        ) {
                            Text("Use push notification instead")
                        }
                    }
                }
            }
        },
        confirmButton = {
            if (mode == "totp") {
                Button(onClick = { onSubmit(code) }, enabled = code.length == 6) { Text("Verify") }
            }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        containerColor = MaterialTheme.colorScheme.surfaceVariant
    )
}
