package com.proitservices.proidentity.access.ui.screen.onboarding

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.outlined.ArrowBack
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.PhonelinkRing
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.proitservices.proidentity.access.R
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.proitservices.proidentity.access.ui.design.ApertureMark
import com.proitservices.proidentity.access.ui.theme.Brand
import com.proitservices.proidentity.access.ui.theme.MonoStyle
import com.proitservices.proidentity.access.ui.viewmodel.SetupStep
import com.proitservices.proidentity.access.ui.viewmodel.SetupUiState
import com.proitservices.proidentity.access.ui.viewmodel.SetupViewModel
import java.net.URI

// ── Onboarding ────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun OnboardingScreen(vm: SetupViewModel) {
    val s by vm.uiState.collectAsStateWithLifecycle()
    BackHandler(enabled = s.step != SetupStep.MODE) { vm.goBack() }

    Column(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        if (s.step != SetupStep.MODE) {
            TopAppBar(
                title = { Text(stepLabel(s.step), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant) },
                navigationIcon = {
                    IconButton(onClick = vm::goBack) {
                        Icon(Icons.AutoMirrored.Outlined.ArrowBack, contentDescription = stringResource(R.string.common_back))
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.background),
            )
        } else {
            Spacer(Modifier.statusBarsPadding())
        }
        AnimatedContent(
            targetState = s.step,
            transitionSpec = {
                val forward = targetState.ordinal > initialState.ordinal
                (slideInHorizontally(tween(260)) { if (forward) it / 5 else -it / 5 } + fadeIn(tween(220)))
                    .togetherWith(slideOutHorizontally(tween(260)) { if (forward) -it / 5 else it / 5 } + fadeOut(tween(160)))
            },
            label = "onboardingStep",
            modifier = Modifier.weight(1f),
        ) { step ->
            when (step) {
                SetupStep.MODE -> ModeStep(onManaged = vm::chooseManaged, onStandalone = vm::chooseStandalone)
                SetupStep.SERVER -> ServerStep(s, vm)
                SetupStep.REGISTER -> RegisterStep(s, vm)
                SetupStep.LOGIN -> SignInForm(s, vm)
                SetupStep.DONE -> Box(Modifier.fillMaxSize())
            }
        }
    }
}

@Composable
private fun stepLabel(step: SetupStep) = when (step) {
    SetupStep.SERVER -> stringResource(R.string.onboarding_step_of, 1, 3)
    SetupStep.REGISTER -> stringResource(R.string.onboarding_step_of, 2, 3)
    SetupStep.LOGIN -> stringResource(R.string.onboarding_step_of, 3, 3)
    else -> ""
}

/**
 * Shared step layout: scrollable content, primary action pinned to the bottom
 * thumb zone and lifted above the keyboard.
 */
@Composable
private fun StepLayout(
    title: String,
    body: String,
    primaryLabel: String?,
    onPrimary: () -> Unit,
    loading: Boolean = false,
    primaryEnabled: Boolean = true,
    header: (@Composable () -> Unit)? = null,
    footer: (@Composable () -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(Modifier.fillMaxSize().imePadding()) {
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 24.dp),
        ) {
            Spacer(Modifier.height(12.dp))
            header?.invoke()
            Text(title, style = MaterialTheme.typography.headlineMedium, modifier = Modifier.semantics { heading() })
            Spacer(Modifier.height(8.dp))
            Text(body, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(28.dp))
            content()
            Spacer(Modifier.height(24.dp))
        }
        Column(
            Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = 24.dp, vertical = 12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (primaryLabel != null) {
                Button(
                    onClick = onPrimary,
                    enabled = primaryEnabled && !loading,
                    shape = MaterialTheme.shapes.large,
                    modifier = Modifier.fillMaxWidth().height(56.dp),
                ) {
                    if (loading) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp, color = MaterialTheme.colorScheme.onPrimary)
                    } else {
                        Text(primaryLabel, style = MaterialTheme.typography.titleMedium)
                    }
                }
            }
            footer?.invoke()
        }
    }
}

@Composable
private fun ErrorText(error: String?) {
    if (error.isNullOrBlank()) return
    Text(
        error,
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.error,
        modifier = Modifier.padding(top = 12.dp),
    )
}

@Composable
private fun ModeStep(onManaged: () -> Unit, onStandalone: () -> Unit) {
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .navigationBarsPadding()
            .padding(horizontal = 24.dp),
    ) {
        Spacer(Modifier.height(48.dp))
        Icon(ApertureMark, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(72.dp))
        Spacer(Modifier.height(28.dp))
        Text(stringResource(R.string.onboarding_welcome_title), style = MaterialTheme.typography.headlineMedium, modifier = Modifier.semantics { heading() })
        Spacer(Modifier.height(10.dp))
        Text(
            stringResource(R.string.onboarding_welcome_body),
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.weight(1f).heightIn(min = 32.dp))
        OptionCard(
            icon = Icons.Outlined.Dns,
            title = stringResource(R.string.onboarding_mode_managed_title),
            body = stringResource(R.string.onboarding_mode_managed_body),
            recommended = true,
            onClick = onManaged,
        )
        Spacer(Modifier.height(12.dp))
        OptionCard(
            icon = Icons.Outlined.Description,
            title = stringResource(R.string.onboarding_mode_standalone_title),
            body = stringResource(R.string.onboarding_mode_standalone_body),
            onClick = onStandalone,
        )
        Spacer(Modifier.height(24.dp))
    }
}

@Composable
private fun OptionCard(icon: ImageVector, title: String, body: String, onClick: () -> Unit, recommended: Boolean = false) {
    Surface(
        onClick = onClick,
        shape = MaterialTheme.shapes.large,
        color = if (recommended) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainerHigh,
        modifier = Modifier.fillMaxWidth(),
    ) {
        Row(Modifier.heightIn(min = 88.dp).padding(18.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(
                Modifier.size(44.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surface.copy(alpha = 0.6f)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
            }
            Spacer(Modifier.width(16.dp))
            Column(Modifier.weight(1f)) {
                Text(title, style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.height(2.dp))
                Text(body, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Composable
private fun ServerStep(s: SetupUiState, vm: SetupViewModel) {
    StepLayout(
        title = stringResource(R.string.onboarding_server_title),
        body = stringResource(R.string.onboarding_server_body),
        primaryLabel = stringResource(R.string.common_continue),
        onPrimary = vm::submitServerUrl,
        loading = s.isLoading,
        primaryEnabled = s.serverUrl.isNotBlank(),
    ) {
        OutlinedTextField(
            value = s.serverUrl,
            onValueChange = { vm.setServerUrl(it); vm.clearError() },
            label = { Text(stringResource(R.string.onboarding_server_address)) },
            placeholder = { Text(stringResource(R.string.onboarding_server_placeholder)) },
            leadingIcon = { Icon(Icons.Outlined.Link, contentDescription = null) },
            singleLine = true,
            isError = s.error != null,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go, autoCorrectEnabled = false),
            keyboardActions = KeyboardActions(onGo = { vm.submitServerUrl() }),
            modifier = Modifier.fillMaxWidth(),
        )
        ErrorText(s.error)
    }
}

@Composable
private fun RegisterStep(s: SetupUiState, vm: SetupViewModel) {
    StepLayout(
        title = stringResource(R.string.onboarding_register_title),
        body = stringResource(R.string.onboarding_register_body, hostOf(s.serverUrl)),
        primaryLabel = stringResource(R.string.onboarding_register_button),
        onPrimary = vm::registerDevice,
        loading = s.isLoading,
    ) {
        OutlinedTextField(
            value = s.deviceName,
            onValueChange = { vm.setDeviceName(it); vm.clearError() },
            label = { Text(stringResource(R.string.onboarding_device_name)) },
            supportingText = { Text(stringResource(R.string.onboarding_device_name_hint)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { vm.registerDevice() }),
            modifier = Modifier.fillMaxWidth(),
        )
        ErrorText(s.error)
    }
}

// ── Sign in (onboarding step 3 and soft re-login) ─────────────────────────

/** Soft re-login screen: the device and server are kept, only credentials are asked. */
@Composable
fun SignInScreen(vm: SetupViewModel, onContinueSignedOut: () -> Unit) {
    val s by vm.uiState.collectAsStateWithLifecycle()
    Column(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background).statusBarsPadding()) {
        SignInForm(
            s, vm,
            footer = if (s.loginMode == "credentials") {
                {
                    TextButton(onClick = onContinueSignedOut, modifier = Modifier.padding(top = 4.dp).heightIn(min = 48.dp)) {
                        Text(stringResource(R.string.signin_continue_signed_out))
                    }
                }
            } else null,
        )
    }
}

@Composable
fun SignInForm(s: SetupUiState, vm: SetupViewModel, footer: (@Composable () -> Unit)? = null) {
    when (s.loginMode) {
        "totp" -> TotpForm(s, vm)
        "push" -> PushForm(s, vm)
        else -> CredentialsForm(s, vm, footer)
    }
}

@Composable
private fun ServerChip(url: String) {
    if (url.isBlank()) return
    Surface(color = MaterialTheme.colorScheme.surfaceContainerHigh, shape = MaterialTheme.shapes.small) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Outlined.Dns, contentDescription = null, modifier = Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.width(8.dp))
            Text(hostOf(url), style = MonoStyle, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun CredentialsForm(s: SetupUiState, vm: SetupViewModel, footer: (@Composable () -> Unit)?) {
    var showPassword by rememberSaveable { mutableStateOf(false) }
    val returning = s.isReauth && s.username.isNotBlank()
    StepLayout(
        title = if (returning) stringResource(R.string.signin_welcome_back) else stringResource(R.string.common_sign_in),
        body = if (returning) stringResource(R.string.signin_reauth_body)
               else stringResource(R.string.signin_body),
        primaryLabel = stringResource(R.string.common_sign_in),
        onPrimary = vm::login,
        loading = s.isLoading,
        primaryEnabled = s.username.isNotBlank() && s.password.isNotBlank(),
        header = if (s.isReauth) ({
            Icon(ApertureMark, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(48.dp))
            Spacer(Modifier.height(20.dp))
        }) else null,
        footer = footer,
    ) {
        ServerChip(s.serverUrl)
        Spacer(Modifier.height(20.dp))
        OutlinedTextField(
            value = s.username,
            onValueChange = { vm.setUsername(it); vm.clearError() },
            label = { Text(stringResource(R.string.signin_username)) },
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next, autoCorrectEnabled = false),
            modifier = Modifier.fillMaxWidth().semantics { contentType = ContentType.Username },
        )
        Spacer(Modifier.height(12.dp))
        OutlinedTextField(
            value = s.password,
            onValueChange = { vm.setPassword(it); vm.clearError() },
            label = { Text(stringResource(R.string.signin_password)) },
            singleLine = true,
            isError = s.error != null,
            visualTransformation = if (showPassword) VisualTransformation.None else PasswordVisualTransformation(),
            trailingIcon = {
                IconButton(onClick = { showPassword = !showPassword }) {
                    Icon(
                        if (showPassword) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility,
                        contentDescription = if (showPassword) stringResource(R.string.signin_hide_password) else stringResource(R.string.signin_show_password),
                    )
                }
            },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { vm.login() }),
            modifier = Modifier.fillMaxWidth().semantics { contentType = ContentType.Password },
        )
        ErrorText(s.error)
    }
}

@Composable
private fun TotpForm(s: SetupUiState, vm: SetupViewModel) {
    LaunchedEffect(s.totpCode) {
        if (s.totpCode.length == 6 && !s.isLoading) vm.loginWithTotp()
    }
    StepLayout(
        title = stringResource(R.string.signin_totp_title),
        body = stringResource(R.string.signin_totp_body),
        primaryLabel = stringResource(R.string.signin_verify),
        onPrimary = vm::loginWithTotp,
        loading = s.isLoading,
        primaryEnabled = s.totpCode.length == 6,
        footer = if (s.pushAuthEnabled) ({
            TextButton(onClick = vm::switchToPush, modifier = Modifier.heightIn(min = 48.dp)) {
                Text(stringResource(R.string.signin_use_push_instead))
            }
        }) else null,
    ) {
        OtpField(value = s.totpCode, onValueChange = { vm.setTotpCode(it); vm.clearError() }, isError = s.error != null)
        ErrorText(s.error)
    }
}

/** Six-digit code field: numeric keyboard, one-time-code autofill, large mono digits. */
@Composable
fun OtpField(value: String, onValueChange: (String) -> Unit, modifier: Modifier = Modifier, isError: Boolean = false) {
    OutlinedTextField(
        value = value,
        onValueChange = { onValueChange(it.filter(Char::isDigit).take(6)) },
        placeholder = { Text(stringResource(R.string.otp_placeholder), style = otpStyle(), modifier = Modifier.fillMaxWidth(), textAlign = TextAlign.Center) },
        textStyle = otpStyle().copy(textAlign = TextAlign.Center),
        singleLine = true,
        isError = isError,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Done),
        modifier = modifier.fillMaxWidth().semantics { contentType = ContentType.SmsOtpCode },
    )
}

@Composable
private fun otpStyle() = TextStyle(fontFamily = FontFamily.Monospace, fontSize = 28.sp, letterSpacing = 8.sp)

@Composable
private fun PushForm(s: SetupUiState, vm: SetupViewModel) {
    PushApproval(
        status = s.pushStatus,
        loading = s.isLoading,
        onRetry = vm::switchToPush,
        onUseCode = vm::switchToTotp,
    )
}

/** Push-approval view shared by sign-in and the connect prompt. */
@Composable
fun PushApproval(status: String, loading: Boolean, onRetry: () -> Unit, onUseCode: () -> Unit) {
    val failed = status == "denied" || status == "expired"
    val (title, body) = when (status) {
        "approved" -> stringResource(R.string.push_approved_title) to stringResource(R.string.push_approved_body)
        "denied" -> stringResource(R.string.push_denied_title) to stringResource(R.string.push_denied_body)
        "expired" -> stringResource(R.string.push_expired_title) to stringResource(R.string.push_expired_body)
        else -> stringResource(R.string.push_pending_title) to stringResource(R.string.push_pending_body)
    }
    StepLayout(
        title = title,
        body = body,
        primaryLabel = if (failed) stringResource(R.string.common_send_again) else null,
        onPrimary = onRetry,
        loading = loading,
        footer = if (status != "approved") ({
            TextButton(onClick = onUseCode, modifier = Modifier.heightIn(min = 48.dp)) { Text(stringResource(R.string.common_enter_code_instead)) }
        }) else null,
    ) {
        val pulse = rememberInfiniteTransition(label = "push")
        val alpha by pulse.animateFloat(0.45f, 1f, infiniteRepeatable(tween(900), RepeatMode.Reverse), label = "pushAlpha")
        val waiting = !failed && status != "approved"
        Box(Modifier.fillMaxWidth().padding(vertical = 24.dp), contentAlignment = Alignment.Center) {
            Box(
                Modifier
                    .size(112.dp)
                    .clip(CircleShape)
                    .background(
                        when {
                            status == "approved" -> Brand.status.connectedContainer
                            failed -> Brand.status.warningContainer
                            else -> MaterialTheme.colorScheme.primaryContainer
                        },
                    ),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    Icons.Outlined.PhonelinkRing, contentDescription = null,
                    tint = when {
                        status == "approved" -> Brand.status.connected
                        failed -> Brand.status.warning
                        else -> MaterialTheme.colorScheme.primary
                    },
                    modifier = Modifier.size(48.dp).alpha(if (waiting) alpha else 1f),
                )
            }
        }
        if (waiting) {
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(10.dp))
                Text(stringResource(R.string.push_waiting), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

internal fun hostOf(url: String): String =
    runCatching { URI(url.trim()).host }.getOrNull()?.takeIf { it.isNotBlank() } ?: url.trim().removePrefix("https://").removePrefix("http://").trimEnd('/')
